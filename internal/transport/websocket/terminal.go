package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"

	terminalApplication "switchyard.dev/switchyard/internal/terminal/application"
	terminalDomain "switchyard.dev/switchyard/internal/terminal/domain"
)

const maximumTerminalInputBytes = 64 << 10

type applicationTerminalSessions interface {
	Attach(context.Context, string, terminalDomain.Owner) (*terminalApplication.Attachment, error)
	Get(context.Context, string, terminalDomain.Owner) (terminalDomain.Session, error)
}

type terminalStream interface {
	Write([]byte) error
	Resize(terminalDomain.Size) error
	Close()
	SnapshotBytes() []byte
	OutputBytes() <-chan []byte
}

type terminalSessions interface {
	Attach(context.Context, string, terminalDomain.Owner) (terminalStream, error)
	Get(context.Context, string, terminalDomain.Owner) (terminalDomain.Session, error)
}

type applicationTerminalAdapter struct{ sessions applicationTerminalSessions }

func (a applicationTerminalAdapter) Attach(ctx context.Context, id string, owner terminalDomain.Owner) (terminalStream, error) {
	return a.sessions.Attach(ctx, id, owner)
}

func (a applicationTerminalAdapter) Get(ctx context.Context, id string, owner terminalDomain.Owner) (terminalDomain.Session, error) {
	return a.sessions.Get(ctx, id, owner)
}

// OwnerResolver translates authenticated HTTP request context into a terminal
// application owner. Authentication remains owned by the HTTP middleware.
type OwnerResolver func(context.Context) terminalDomain.Owner

// Terminal carries binary PTY input/output and bounded JSON control messages.
type Terminal struct {
	sessions terminalSessions
	owner    OwnerResolver
}

// NewTerminal constructs the authenticated PTY stream adapter.
func NewTerminal(sessions applicationTerminalSessions, owner OwnerResolver) *Terminal {
	return &Terminal{sessions: applicationTerminalAdapter{sessions: sessions}, owner: owner}
}

type terminalClientControl struct {
	Type    string `json:"type"`
	Columns uint16 `json:"columns,omitempty"`
	Rows    uint16 `json:"rows,omitempty"`
}

type terminalServerControl struct {
	Type              string                 `json:"type"`
	SessionID         string                 `json:"sessionId"`
	Status            terminalDomain.Status  `json:"status"`
	PersistencePolicy string                 `json:"persistencePolicy,omitempty"`
	CapturePolicy     string                 `json:"capturePolicy,omitempty"`
	Reason            string                 `json:"reason,omitempty"`
	ExitCode          *int                   `json:"exitCode,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}

// ServeHTTP attaches after authorization, then upgrades to a same-origin
// binary/text WebSocket. Disconnecting detaches; it does not terminate the PTY.
func (t *Terminal) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	owner := t.owner(r.Context())
	attachment, err := t.sessions.Attach(r.Context(), sessionID, owner)
	if err != nil {
		writeTerminalStreamError(w, err)
		return
	}
	defer attachment.Close()

	connection, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"localhost:*", "127.0.0.1:*", "[::1]:*"},
	})
	if err != nil {
		return
	}
	defer func() { _ = connection.CloseNow() }()
	connection.SetReadLimit(maximumTerminalInputBytes)

	streamCtx, cancel := context.WithCancel(r.Context())
	defer cancel()
	session, err := t.sessions.Get(streamCtx, sessionID, owner)
	if err != nil {
		_ = connection.Close(websocket.StatusInternalError, "session metadata unavailable")
		return
	}
	if err := writeTerminalControl(streamCtx, connection, terminalServerControl{
		Type: "ready", SessionID: session.ID, Status: session.Status,
		PersistencePolicy: session.PersistencePolicy, CapturePolicy: session.CapturePolicy,
		Metadata: map[string]interface{}{"kind": session.Kind, "workingDirectory": session.WorkingDirectory, "displayName": session.DisplayName},
	}); err != nil {
		return
	}
	if snapshot := attachment.SnapshotBytes(); len(snapshot) > 0 {
		if err := connection.Write(streamCtx, websocket.MessageBinary, snapshot); err != nil {
			return
		}
	}

	readErrors := make(chan error, 1)
	go func() { readErrors <- readTerminalInput(streamCtx, connection, attachment) }()
	for {
		select {
		case <-streamCtx.Done():
			return
		case err := <-readErrors:
			if err != nil && !isNormalTerminalClose(err) {
				_ = connection.Close(websocket.StatusPolicyViolation, "terminal input rejected")
			}
			return
		case output, ok := <-attachment.OutputBytes():
			if ok {
				if err := connection.Write(streamCtx, websocket.MessageBinary, output); err != nil {
					return
				}
				continue
			}
			final, getErr := t.sessions.Get(context.WithoutCancel(streamCtx), sessionID, owner)
			if getErr != nil {
				return
			}
			reason := "process_finished"
			status := websocket.StatusNormalClosure
			if final.Active() {
				reason = "slow_consumer_detached"
				status = websocket.StatusPolicyViolation
			}
			_ = writeTerminalControl(streamCtx, connection, terminalServerControl{
				Type: "exit", SessionID: sessionID, Status: final.Status, Reason: reason, ExitCode: final.ExitCode,
			})
			_ = connection.Close(status, reason)
			return
		}
	}
}

func readTerminalInput(ctx context.Context, connection *websocket.Conn, attachment terminalStream) error {
	for {
		messageType, value, err := connection.Read(ctx)
		if err != nil {
			return err
		}
		switch messageType {
		case websocket.MessageBinary:
			if len(value) > maximumTerminalInputBytes {
				return errors.New("terminal input exceeds bound")
			}
			if err := attachment.Write(value); err != nil {
				return err
			}
		case websocket.MessageText:
			var control terminalClientControl
			decoder := json.NewDecoder(strings.NewReader(string(value)))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&control); err != nil || control.Type != "resize" {
				return errors.New("terminal control message is invalid")
			}
			if err := attachment.Resize(terminalDomain.Size{Columns: control.Columns, Rows: control.Rows}); err != nil {
				return err
			}
		default:
			return errors.New("terminal message type is unsupported")
		}
	}
}

func writeTerminalControl(ctx context.Context, connection *websocket.Conn, message terminalServerControl) error {
	value, err := json.Marshal(message)
	if err != nil {
		return err
	}
	return connection.Write(ctx, websocket.MessageText, value)
}

func writeTerminalStreamError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, terminalApplication.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, terminalApplication.ErrOwnerMismatch):
		status = http.StatusForbidden
	case errors.Is(err, terminalApplication.ErrNotActive):
		status = http.StatusConflict
	}
	http.Error(w, http.StatusText(status), status)
}

func isNormalTerminalClose(err error) bool {
	status := websocket.CloseStatus(err)
	return status == websocket.StatusNormalClosure || status == websocket.StatusGoingAway || errors.Is(err, context.Canceled)
}
