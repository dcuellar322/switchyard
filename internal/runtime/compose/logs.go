package compose

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"golang.org/x/sync/errgroup"
	"switchyard.dev/switchyard/internal/runtime/domain"
)

func (d *Driver) streamLogs(ctx context.Context, request domain.LogRequest, config normalizedConfig, sink domain.LogSink) error {
	engine, _, _, err := d.engine.Connect(ctx, config.Connection)
	if err != nil {
		return err
	}
	defer func() { _ = engine.Close() }()
	containers, err := engine.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: logFilters(config.ProjectName, request.Service)})
	if err != nil {
		return fmt.Errorf("list Compose containers for logs: %w", err)
	}
	group, groupCtx := errgroup.WithContext(ctx)
	for _, item := range composeContainers(containers.Items, config.ProjectName, config.Services) {
		item := item
		group.Go(func() error { return streamContainerLogs(groupCtx, engine, request, item, sink) })
	}
	return group.Wait()
}

func logFilters(projectName, service string) client.Filters {
	filters := projectFilters(projectName)
	if service != "" {
		filters = filters.Add("label", labelService+"="+service)
	}
	return filters
}

func streamContainerLogs(ctx context.Context, engine engineClient, request domain.LogRequest, item container.Summary, sink domain.LogSink) error {
	inspect, err := engine.ContainerInspect(ctx, item.ID, client.ContainerInspectOptions{})
	if err != nil {
		return fmt.Errorf("inspect Compose container for logs: %w", err)
	}
	stream, err := engine.ContainerLogs(ctx, item.ID, client.ContainerLogsOptions{
		ShowStdout: true, ShowStderr: true, Since: request.Since,
		Timestamps: true, Follow: request.Follow, Tail: strconv.Itoa(request.Tail),
	})
	if err != nil {
		return fmt.Errorf("open Compose container logs: %w", err)
	}
	defer func() { _ = stream.Close() }()
	serviceID := productServiceID(request.Project, item.Labels[labelService])
	stdout := newLogLineWriter(ctx, sink, request.Project.ProjectID, serviceID, item.ID, "stdout")
	if inspect.Container.Config != nil && inspect.Container.Config.Tty {
		_, err = stdout.ReadFrom(stream)
		return errorsWithFlush(err, stdout.Flush())
	}
	stderr := newLogLineWriter(ctx, sink, request.Project.ProjectID, serviceID, item.ID, "stderr")
	_, err = stdcopy.StdCopy(stdout, stderr, stream)
	return errorsWithFlush(err, stdout.Flush(), stderr.Flush())
}

type logLineWriter struct {
	ctx       context.Context
	sink      domain.LogSink
	projectID string
	serviceID string
	container string
	stream    string
	mu        sync.Mutex
	buffer    []byte
}

func newLogLineWriter(ctx context.Context, sink domain.LogSink, projectID, serviceID, container, stream string) *logLineWriter {
	return &logLineWriter{ctx: ctx, sink: sink, projectID: projectID, serviceID: serviceID, container: container, stream: stream}
}

func (w *logLineWriter) Write(value []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buffer = append(w.buffer, value...)
	if len(w.buffer) > 1<<20 {
		return 0, errorsWithFlush(errors.New("docker log line exceeds 1 MiB"))
	}
	for {
		index := bytes.IndexByte(w.buffer, '\n')
		if index < 0 {
			break
		}
		line := append([]byte(nil), w.buffer[:index]...)
		w.buffer = w.buffer[index+1:]
		if err := w.emit(line); err != nil {
			return 0, err
		}
	}
	return len(value), nil
}

func (w *logLineWriter) ReadFrom(reader io.Reader) (int64, error) {
	return io.Copy(w, reader)
}

func (w *logLineWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.buffer) == 0 {
		return nil
	}
	line := append([]byte(nil), w.buffer...)
	w.buffer = nil
	return w.emit(line)
}

func (w *logLineWriter) emit(line []byte) error {
	timestamp, message := parseLogTimestamp(string(line))
	return w.sink.WriteLog(w.ctx, domain.LogEntry{
		Timestamp: timestamp, ProjectID: w.projectID, ServiceID: w.serviceID,
		RunID: "docker:" + w.container, Source: "docker", Stream: w.stream,
		Level: detectLevel(message), Message: message,
		Attributes: map[string]string{"containerId": w.container},
	})
}

func parseLogTimestamp(line string) (time.Time, string) {
	stamp, message, found := strings.Cut(strings.TrimSuffix(line, "\r"), " ")
	if found {
		if parsed, err := time.Parse(time.RFC3339Nano, stamp); err == nil {
			return parsed.UTC(), message
		}
	}
	return time.Now().UTC(), strings.TrimSuffix(line, "\r")
}

func detectLevel(message string) string {
	lower := strings.ToLower(strings.TrimSpace(message))
	for _, level := range []string{"trace", "debug", "info", "warn", "error", "fatal"} {
		if strings.HasPrefix(lower, level) || strings.Contains(lower, `"level":"`+level+`"`) {
			return level
		}
	}
	fields := strings.Fields(lower)
	if len(fields) > 6 {
		fields = fields[:6]
	}
	for _, field := range fields {
		field = strings.Trim(field, "[]{}():,\"")
		switch field {
		case "trace", "debug", "info", "warn", "error", "fatal":
			return field
		case "warning":
			return "warn"
		}
	}
	return "unknown"
}

func errorsWithFlush(values ...error) error {
	var result error
	for _, value := range values {
		if value != nil {
			result = errors.Join(result, value)
		}
	}
	return result
}
