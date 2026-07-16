package compose

import (
	"bytes"
	"context"
	"io"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

type fakeConnector struct {
	engine  engineClient
	ping    client.PingResult
	version client.ServerVersionResult
	err     error
}

func (f fakeConnector) Connect(context.Context, dockerConnection) (engineClient, client.PingResult, client.ServerVersionResult, error) {
	return f.engine, f.ping, f.version, f.err
}

type fakeEngine struct {
	containers []container.Summary
	inspects   map[string]container.InspectResponse
	logs       map[string][]byte
	stats      map[string][]byte
	events     client.EventsResult
}

func (*fakeEngine) Ping(context.Context, client.PingOptions) (client.PingResult, error) {
	return client.PingResult{}, nil
}

func (*fakeEngine) ServerVersion(context.Context, client.ServerVersionOptions) (client.ServerVersionResult, error) {
	return client.ServerVersionResult{}, nil
}

func (f *fakeEngine) ContainerList(context.Context, client.ContainerListOptions) (client.ContainerListResult, error) {
	return client.ContainerListResult{Items: f.containers}, nil
}

func (f *fakeEngine) ContainerInspect(_ context.Context, id string, _ client.ContainerInspectOptions) (client.ContainerInspectResult, error) {
	return client.ContainerInspectResult{Container: f.inspects[id]}, nil
}

func (f *fakeEngine) ContainerLogs(_ context.Context, id string, _ client.ContainerLogsOptions) (client.ContainerLogsResult, error) {
	return io.NopCloser(bytes.NewReader(f.logs[id])), nil
}

func (f *fakeEngine) ContainerStats(_ context.Context, id string, _ client.ContainerStatsOptions) (client.ContainerStatsResult, error) {
	return client.ContainerStatsResult{Body: io.NopCloser(bytes.NewReader(f.stats[id]))}, nil
}

func (f *fakeEngine) Events(context.Context, client.EventsListOptions) client.EventsResult {
	return f.events
}

func (*fakeEngine) Close() error { return nil }
