// Package daemonlog persists bounded, redacted control-plane events for local
// troubleshooting. It never receives project application output.
package daemonlog

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

const fileMode = 0o600

// Redact removes credential-bearing text before persistence.
type Redact func(string) (string, bool)

// FileHandler is a slog handler backed by a two-segment private file.
type FileHandler struct {
	inner  slog.Handler
	writer *rotatingWriter
	redact Redact
}

// Open creates a JSON log handler that rotates at maximumBytes.
func Open(path string, maximumBytes int64, redact Redact) (*FileHandler, error) {
	if maximumBytes < 64<<10 || redact == nil {
		return nil, errors.New("daemon log bounds and redactor are required")
	}
	writer, err := openRotatingWriter(path, maximumBytes)
	if err != nil {
		return nil, err
	}
	return &FileHandler{
		inner:  slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: slog.LevelDebug}),
		writer: writer,
		redact: redact,
	}, nil
}

// Enabled delegates level selection to the JSON handler.
func (h *FileHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle redacts messages and attributes before the JSON encoder sees them.
func (h *FileHandler) Handle(ctx context.Context, record slog.Record) error {
	message, _ := h.redact(record.Message)
	copyRecord := slog.NewRecord(record.Time, record.Level, message, record.PC)
	record.Attrs(func(attribute slog.Attr) bool {
		copyRecord.AddAttrs(h.redactAttribute(attribute))
		return true
	})
	return h.inner.Handle(ctx, copyRecord)
}

// WithAttrs returns a derived handler sharing the same bounded writer.
func (h *FileHandler) WithAttrs(attributes []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, 0, len(attributes))
	for _, attribute := range attributes {
		redacted = append(redacted, h.redactAttribute(attribute))
	}
	return &FileHandler{inner: h.inner.WithAttrs(redacted), writer: h.writer, redact: h.redact}
}

// WithGroup returns a derived grouped handler.
func (h *FileHandler) WithGroup(name string) slog.Handler {
	return &FileHandler{inner: h.inner.WithGroup(name), writer: h.writer, redact: h.redact}
}

// Close flushes the private log file.
func (h *FileHandler) Close() error { return h.writer.close() }

func (h *FileHandler) redactAttribute(attribute slog.Attr) slog.Attr {
	value := attribute.Value.Resolve()
	switch value.Kind() {
	case slog.KindString:
		redacted, _ := h.redact(value.String())
		return slog.String(attribute.Key, redacted)
	case slog.KindAny:
		redacted, _ := h.redact(fmt.Sprint(value.Any()))
		return slog.String(attribute.Key, redacted)
	case slog.KindGroup:
		group := value.Group()
		for index := range group {
			group[index] = h.redactAttribute(group[index])
		}
		return slog.Attr{Key: attribute.Key, Value: slog.GroupValue(group...)}
	case slog.KindBool, slog.KindDuration, slog.KindFloat64, slog.KindInt64, slog.KindTime, slog.KindUint64:
		return attribute
	case slog.KindLogValuer:
		// Value.Resolve above normally removes this kind. Preserve the resolved
		// value as text if a custom implementation still reports it.
		redacted, _ := h.redact(fmt.Sprint(value.Any()))
		return slog.String(attribute.Key, redacted)
	default:
		return attribute
	}
}

// Tee sends each record to both handlers without hiding either failure.
func Tee(primary, secondary slog.Handler) slog.Handler {
	return teeHandler{primary: primary, secondary: secondary}
}

type teeHandler struct{ primary, secondary slog.Handler }

func (h teeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.primary.Enabled(ctx, level) || h.secondary.Enabled(ctx, level)
}

func (h teeHandler) Handle(ctx context.Context, record slog.Record) error {
	var first, second error
	if h.primary.Enabled(ctx, record.Level) {
		first = h.primary.Handle(ctx, record.Clone())
	}
	if h.secondary.Enabled(ctx, record.Level) {
		second = h.secondary.Handle(ctx, record.Clone())
	}
	return errors.Join(first, second)
}

func (h teeHandler) WithAttrs(attributes []slog.Attr) slog.Handler {
	return teeHandler{primary: h.primary.WithAttrs(attributes), secondary: h.secondary.WithAttrs(attributes)}
}

func (h teeHandler) WithGroup(name string) slog.Handler {
	return teeHandler{primary: h.primary.WithGroup(name), secondary: h.secondary.WithGroup(name)}
}

type rotatingWriter struct {
	mu           sync.Mutex
	path         string
	file         *os.File
	size         int64
	maximumBytes int64
}

func openRotatingWriter(path string, maximumBytes int64) (*rotatingWriter, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create internal log directory: %w", err)
	}
	if err := requireRegularOrMissing(path); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, fileMode)
	if err != nil {
		return nil, fmt.Errorf("open internal daemon log: %w", err)
	}
	if err := file.Chmod(fileMode); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("restrict internal daemon log: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("inspect internal daemon log: %w", err)
	}
	return &rotatingWriter{path: path, file: file, size: info.Size(), maximumBytes: maximumBytes}, nil
}

func (w *rotatingWriter) Write(value []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return 0, os.ErrClosed
	}
	if w.size > 0 && w.size+int64(len(value)) > w.maximumBytes {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	written, err := w.file.Write(value)
	w.size += int64(written)
	return written, err
}

func (w *rotatingWriter) rotate() error {
	if err := w.file.Close(); err != nil {
		return fmt.Errorf("close full internal daemon log: %w", err)
	}
	w.file = nil
	previous := w.path + ".1"
	if err := os.Remove(previous); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove expired internal daemon log: %w", err)
	}
	if err := os.Rename(w.path, previous); err != nil {
		return fmt.Errorf("rotate internal daemon log: %w", err)
	}
	file, err := os.OpenFile(w.path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, fileMode)
	if err != nil {
		return fmt.Errorf("create rotated internal daemon log: %w", err)
	}
	w.file, w.size = file, 0
	return nil
}

func (w *rotatingWriter) close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

func requireRegularOrMissing(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect internal daemon log: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("internal daemon log path is not a regular file: %s", path)
	}
	return nil
}

var _ io.Writer = (*rotatingWriter)(nil)
