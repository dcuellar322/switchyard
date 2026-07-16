package compose

import (
	"bytes"
	"context"
	"encoding/binary"
	"testing"
	"time"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"switchyard.dev/switchyard/internal/runtime/domain"
)

func TestStreamContainerLogsPreservesIdentityAndStreams(t *testing.T) {
	t.Parallel()
	var multiplexed bytes.Buffer
	writeMultiplexed(&multiplexed, stdcopy.Stdout, []byte("2026-07-16T04:00:00.123Z info ready\n"))
	writeMultiplexed(&multiplexed, stdcopy.Stderr, []byte("2026-07-16T04:00:01.123Z error failed\n"))
	engine := &fakeEngine{
		inspects: map[string]container.InspectResponse{"container-1": {Config: &container.Config{}}},
		logs:     map[string][]byte{"container-1": multiplexed.Bytes()},
	}
	sink := &recordingLogSink{}
	request := domain.LogRequest{
		Project: domain.ProjectRuntime{ProjectID: "project-1", Services: []domain.ServiceDeclaration{{ID: "api", RuntimeName: "web"}}}, Tail: 20,
	}
	item := container.Summary{ID: "container-1", Labels: map[string]string{labelService: "web"}}
	if err := streamContainerLogs(context.Background(), engine, request, item, sink); err != nil {
		t.Fatal(err)
	}
	if len(sink.entries) != 2 {
		t.Fatalf("entries = %#v", sink.entries)
	}
	if sink.entries[0].ServiceID != "api" || sink.entries[0].Stream != "stdout" || sink.entries[0].Level != "info" {
		t.Fatalf("stdout = %#v", sink.entries[0])
	}
	if sink.entries[1].RunID != "docker:container-1" || sink.entries[1].Stream != "stderr" || sink.entries[1].Level != "error" {
		t.Fatalf("stderr = %#v", sink.entries[1])
	}
	expected := time.Date(2026, 7, 16, 4, 0, 0, 123_000_000, time.UTC)
	if !sink.entries[0].Timestamp.Equal(expected) {
		t.Fatalf("timestamp = %s", sink.entries[0].Timestamp)
	}
}

func TestDetectLevelAfterApplicationTimestamp(t *testing.T) {
	t.Parallel()
	if level := detectLevel("2026/07/16 08:39:20 info fixture ready"); level != "info" {
		t.Fatalf("level = %q", level)
	}
}

func writeMultiplexed(buffer *bytes.Buffer, stream stdcopy.StdType, value []byte) {
	header := make([]byte, 8)
	header[0] = byte(stream)
	binary.BigEndian.PutUint32(header[4:], uint32(len(value)))
	_, _ = buffer.Write(header)
	_, _ = buffer.Write(value)
}

type recordingLogSink struct {
	entries []domain.LogEntry
}

func (s *recordingLogSink) WriteLog(_ context.Context, entry domain.LogEntry) error {
	s.entries = append(s.entries, entry)
	return nil
}
