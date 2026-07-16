package compose

import (
	"context"
	"testing"
	"time"

	apiEvents "github.com/moby/moby/api/types/events"
	"github.com/moby/moby/client"
	"switchyard.dev/switchyard/internal/runtime/domain"
)

func TestWatchEventsEmitsOnlyCanonicalProjectMembership(t *testing.T) {
	t.Parallel()
	messages := make(chan apiEvents.Message, 2)
	errors := make(chan error)
	messages <- apiEvents.Message{Action: apiEvents.Action("start"), Actor: apiEvents.Actor{
		ID: "container-1", Attributes: map[string]string{labelProject: "fixture", labelService: "web"},
	}, TimeNano: time.Date(2026, 7, 16, 4, 0, 0, 0, time.UTC).UnixNano()}
	messages <- apiEvents.Message{Action: apiEvents.Action("start"), Actor: apiEvents.Actor{
		ID: "container-2", Attributes: map[string]string{labelProject: "other", labelService: "web"},
	}}
	close(messages)
	engine := &fakeEngine{events: client.EventsResult{Messages: messages, Err: errors}}
	driver := &Driver{engine: fakeConnector{engine: engine}}
	sink := &recordingEventSink{}
	if err := driver.watchEvents(context.Background(), normalizedConfig{ProjectName: "fixture"}, sink); err != nil {
		t.Fatal(err)
	}
	if len(sink.events) != 1 || sink.events[0].ProjectIdentity != "fixture" || sink.events[0].ServiceIdentity != "web" {
		t.Fatalf("events = %#v", sink.events)
	}
}

type recordingEventSink struct {
	events []domain.RuntimeEvent
}

func (s *recordingEventSink) WriteRuntimeEvent(_ context.Context, event domain.RuntimeEvent) error {
	s.events = append(s.events, event)
	return nil
}
