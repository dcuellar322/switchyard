package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	session "switchyard.dev/switchyard/internal/session/application"
)

const (
	sessionCookieName = "switchyard_session"
	csrfHeader        = "X-CSRF-Token"
	idempotencyHeader = "Idempotency-Key"
	actorTypeHeader   = "X-Switchyard-Actor-Type"
	actorIDHeader     = "X-Switchyard-Actor-ID"
)

var agentActorPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:/-]{0,127}$`)

type accessKind string

const (
	accessBrowser accessKind = "browser"
	accessIPC     accessKind = "ipc"
)

type securityContextKey struct{}

type requestIdentity struct {
	Access  accessKind
	ActorID string
}

func withAccess(access accessKind, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity := requestIdentity{Access: access, ActorID: string(access)}
		if access == accessIPC && r.Header.Get(actorTypeHeader) != "" {
			if r.Header.Get(actorTypeHeader) != "agent" || !agentActorPattern.MatchString(r.Header.Get(actorIDHeader)) {
				writeProblem(w, r, http.StatusBadRequest, "ACTOR_IDENTITY_INVALID", "Actor identity invalid", "Agent identity headers must contain one bounded agent identifier.")
				return
			}
			identity.Access = accessKind("agent")
			identity.ActorID = r.Header.Get(actorIDHeader)
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), securityContextKey{}, identity)))
	})
}

func withBrowserHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self' ws://127.0.0.1:* ws://localhost:* ws://[::1]:*; img-src 'self' data:; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func identityFrom(ctx context.Context) requestIdentity {
	identity, _ := ctx.Value(securityContextKey{}).(requestIdentity)
	return identity
}

// RequestActor returns the authenticated local actor attached by the transport
// security middleware. It lets sibling stream adapters enforce application
// ownership without knowing browser cookie or IPC header details.
func RequestActor(ctx context.Context) (string, string) {
	identity := identityFrom(ctx)
	return string(identity.Access), identity.ActorID
}

func withBrowserSecurity(sessions sessionService, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/sessions" && r.Method == http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/v1/") && !isProtectedWebSocketPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			writeProblem(w, r, http.StatusUnauthorized, "SESSION_REQUIRED", "Browser session required", "Launch the UI through switchyard ui.")
			return
		}
		active, err := sessions.ValidateSession(cookie.Value)
		if err != nil {
			writeProblem(w, r, http.StatusUnauthorized, "SESSION_INVALID", "Browser session invalid", "Launch the UI again to create a fresh session.")
			return
		}
		if isProtectedWebSocketPath(r.URL.Path) && !sameOrigin(r) {
			writeProblem(w, r, http.StatusForbidden, "ORIGIN_INVALID", "WebSocket origin rejected", "The event stream is same-origin only.")
			return
		}
		if isMutationRequest(r) {
			if _, err := sessions.ValidateMutation(cookie.Value, r.Header.Get(csrfHeader)); err != nil {
				code := "CSRF_INVALID"
				if !errors.Is(err, session.ErrInvalidCSRF) {
					code = "SESSION_INVALID"
				}
				writeProblem(w, r, http.StatusForbidden, code, "Mutation authorization failed", "A valid session and CSRF token are required.")
				return
			}
		}
		identity := identityFrom(r.Context())
		identity.ActorID = active.ID
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), securityContextKey{}, identity)))
	})
}

func isProtectedWebSocketPath(path string) bool {
	return path == "/ws/v1/events" || path == "/ws/v1/logs" ||
		strings.HasPrefix(path, "/ws/v1/terminal/") || strings.HasPrefix(path, "/ws/v1/agent-sessions/")
}

func withIdempotencyKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isMutationRequest(r) && !strings.HasPrefix(r.URL.Path, "/api/v1/auth/") {
			key := r.Header.Get(idempotencyHeader)
			if len(key) < 8 || len(key) > 128 {
				writeProblem(w, r, http.StatusBadRequest, "IDEMPOTENCY_KEY_INVALID", "Idempotency key invalid", "Use an opaque key between 8 and 128 characters.")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func isMutation(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
}

func isMutationRequest(request *http.Request) bool {
	if request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/runtime/plan") {
		return false
	}
	return isMutation(request.Method)
}

func sameOrigin(r *http.Request) bool {
	origin, err := url.Parse(r.Header.Get("Origin"))
	if err != nil || origin.Scheme == "" || origin.Host == "" {
		return false
	}
	return origin.Scheme == "http" && origin.Host == r.Host
}
