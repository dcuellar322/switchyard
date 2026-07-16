package adapters

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"switchyard.dev/switchyard/internal/support/application"
	"switchyard.dev/switchyard/internal/support/domain"
)

// TextRedactor removes configured credential forms from persisted text.
type TextRedactor func(string) (string, bool)

// InternalLogSource reads the dedicated redacted daemon log segments.
type InternalLogSource struct {
	path    string
	dataDir string
	homeDir string
	redact  TextRedactor
}

// NewInternalLogSource creates a bounded support log adapter.
func NewInternalLogSource(dataDir string, redact TextRedactor) (*InternalLogSource, error) {
	if strings.TrimSpace(dataDir) == "" || redact == nil {
		return nil, fmt.Errorf("internal log data directory and redactor are required")
	}
	home, _ := os.UserHomeDir()
	return &InternalLogSource{
		path: filepath.Join(dataDir, "internal.ndjson"), dataDir: filepath.Clean(dataDir), homeDir: filepath.Clean(home), redact: redact,
	}, nil
}

// List returns oldest-to-newest allowlisted records across two bounded segments.
func (s *InternalLogSource) List(ctx context.Context, query application.LogQuery) ([]domain.InternalLogEntry, error) {
	minimum := logPriority(query.MinimumLevel)
	var result []domain.InternalLogEntry
	for _, path := range []string{s.path + ".1", s.path} {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		items, err := s.readFile(ctx, path, minimum)
		if err != nil {
			return nil, err
		}
		result = append(result, items...)
		if len(result) > query.Limit {
			result = result[len(result)-query.Limit:]
		}
	}
	return result, nil
}

func (s *InternalLogSource) readFile(ctx context.Context, path string, minimum int) ([]domain.InternalLogEntry, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect internal log segment: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("internal log segment is not a regular file: %s", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open internal log segment: %w", err)
	}
	defer func() { _ = file.Close() }()

	var entries []domain.InternalLogEntry
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		entry, ok := s.decode(scanner.Bytes())
		if ok && logPriority(entry.Level) >= minimum {
			entries = append(entries, entry)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan internal log segment: %w", err)
	}
	return entries, nil
}

func (s *InternalLogSource) decode(line []byte) (domain.InternalLogEntry, bool) {
	var record map[string]any
	if err := json.Unmarshal(line, &record); err != nil {
		return domain.InternalLogEntry{}, false
	}
	timestamp, err := time.Parse(time.RFC3339Nano, stringValue(record["time"]))
	if err != nil {
		return domain.InternalLogEntry{}, false
	}
	level := strings.ToUpper(stringValue(record["level"]))
	if logPriority(level) < 0 {
		return domain.InternalLogEntry{}, false
	}
	return domain.InternalLogEntry{
		Timestamp: timestamp.UTC(), Level: level,
		Message: s.safe(stringValue(record["msg"])), Component: s.safe(stringValue(record["component"])),
		ErrorCode: s.safe(stringValue(record["error_code"])), Error: s.safe(stringValue(record["error"])),
		ProjectID: s.safe(stringValue(record["project_id"])), OperationID: s.safe(stringValue(record["operation_id"])),
		CorrelationID: s.safe(stringValue(record["correlation_id"])),
	}, true
}

func (s *InternalLogSource) safe(value string) string {
	redacted, _ := s.redact(value)
	if s.dataDir != "." && s.dataDir != "" {
		redacted = strings.ReplaceAll(redacted, s.dataDir, "<data-dir>")
	}
	if s.homeDir != "." && s.homeDir != "" {
		redacted = strings.ReplaceAll(redacted, s.homeDir, "<home>")
	}
	return redacted
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func logPriority(level string) int {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return 0
	case "INFO":
		return 1
	case "WARN", "WARNING":
		return 2
	case "ERROR":
		return 3
	default:
		return -1
	}
}
