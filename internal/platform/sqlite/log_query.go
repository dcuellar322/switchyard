package sqlite

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	observability "switchyard.dev/switchyard/internal/observability/domain"
	runtime "switchyard.dev/switchyard/internal/runtime/domain"
)

type logLocation struct {
	sequence int64
	path     string
	line     int64
}

// QueryLogs reads a bounded project stream from indexed, redacted NDJSON segments.
func (s *LogStore) QueryLogs(ctx context.Context, query observability.LogQuery) ([]runtime.LogEntry, error) {
	locations, err := s.queryLogLocations(ctx, query)
	if err != nil {
		return nil, err
	}
	return readLogLocations(locations)
}

// ReplayLogs returns ordered entries after a cursor and reports a truncated replay window.
func (s *LogStore) ReplayLogs(ctx context.Context, projectID, serviceID string, after int64, limit int) ([]runtime.LogEntry, bool, error) {
	entries, err := s.QueryLogs(ctx, observability.LogQuery{ProjectID: projectID, ServiceID: serviceID, After: after, Limit: limit + 1})
	if err != nil {
		return nil, false, err
	}
	truncated := len(entries) > limit
	if truncated {
		entries = entries[:limit]
	}
	return entries, truncated, nil
}

func (s *LogStore) queryLogLocations(ctx context.Context, query observability.LogQuery) ([]logLocation, error) {
	if query.Limit <= 0 || query.Limit > 10_000 {
		query.Limit = 200
	}
	since := ""
	if !query.Since.IsZero() {
		since = formatTime(query.Since)
	}
	order := "DESC"
	if query.After > 0 {
		order = "ASC"
	}
	statement := `SELECT e.sequence, s.path, e.line_number FROM log_entries e
        JOIN log_segments s ON s.id = e.segment_id
        WHERE e.project_id = ? AND (? = '' OR e.service_id = ?) AND (? = '' OR e.run_id = ?)
          AND (? = '' OR e.operation_id = ?) AND (? = '' OR e.occurred_at >= ?) AND e.sequence > ?
        ORDER BY e.sequence ` + order + ` LIMIT ?`
	rows, err := s.database.connection.QueryContext(ctx, statement, query.ProjectID, query.ServiceID, query.ServiceID,
		query.RunID, query.RunID, query.OperationID, query.OperationID, since, since, query.After, query.Limit)
	if err != nil {
		return nil, fmt.Errorf("query log index: %w", err)
	}
	defer func() { _ = rows.Close() }()
	locations := []logLocation{}
	for rows.Next() {
		var location logLocation
		if err := rows.Scan(&location.sequence, &location.path, &location.line); err != nil {
			return nil, err
		}
		locations = append(locations, location)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if order == "DESC" {
		sort.Slice(locations, func(i, j int) bool { return locations[i].sequence < locations[j].sequence })
	}
	return locations, nil
}

func readLogLocations(locations []logLocation) ([]runtime.LogEntry, error) {
	requested := make(map[string]map[int64]int64)
	for _, location := range locations {
		if requested[location.path] == nil {
			requested[location.path] = map[int64]int64{}
		}
		requested[location.path][location.line] = location.sequence
	}
	bySequence := make(map[int64]runtime.LogEntry, len(locations))
	for path, lines := range requested {
		file, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open indexed log segment: %w", err)
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 64*1024), 2<<20)
		var line int64
		for scanner.Scan() {
			line++
			sequence, wanted := lines[line]
			if !wanted {
				continue
			}
			var entry runtime.LogEntry
			if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
				_ = file.Close()
				return nil, fmt.Errorf("decode indexed log entry: %w", err)
			}
			if entry.Sequence != sequence {
				_ = file.Close()
				return nil, fmt.Errorf("log segment index mismatch at %s:%d", path, line)
			}
			bySequence[sequence] = entry
		}
		scanErr := scanner.Err()
		_ = file.Close()
		if scanErr != nil {
			return nil, scanErr
		}
	}
	entries := make([]runtime.LogEntry, 0, len(locations))
	for _, location := range locations {
		entry, ok := bySequence[location.sequence]
		if !ok {
			return nil, fmt.Errorf("indexed log entry %d is missing", location.sequence)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// ExportLogs writes only already-redacted persisted entries as plain text or NDJSON.
func (s *LogStore) ExportLogs(ctx context.Context, query observability.LogQuery, format string, writer io.Writer) error {
	entries, err := s.QueryLogs(ctx, query)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(writer)
	for _, entry := range entries {
		if format == "ndjson" {
			if err := encoder.Encode(entry); err != nil {
				return err
			}
			continue
		}
		message := strings.ReplaceAll(entry.Message, "\n", "\\n")
		if _, err := fmt.Fprintf(writer, "%s %-12s %-6s %s\n", entry.Timestamp.Format("2006-01-02T15:04:05.000Z07:00"), entry.ServiceID, entry.Stream, message); err != nil {
			return err
		}
	}
	return nil
}
