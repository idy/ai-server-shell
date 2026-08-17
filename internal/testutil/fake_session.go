package testutil

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/idy/ai-server-shell/backend"
)

// FakeSessionBackend creates deterministic buffered sessions for SDK tests.
type FakeSessionBackend struct{}

func (*FakeSessionBackend) OpenSession(_ context.Context, request backend.SessionRequest) (backend.Session, error) {
	session := &fakeSession{
		surface: request.Surface, queue: make(chan backend.Event, 8),
		events: make(chan backend.Event, 8), done: make(chan struct{}),
	}
	if request.Surface == backend.SessionRealtime {
		session.queue <- event("session.created", `{"type":"session.created","event_id":"event_test","session":{"id":"sess_test","object":"realtime.session","type":"realtime"}}`)
	}
	go session.run()
	return session, nil
}

type fakeSession struct {
	surface backend.SessionSurface
	queue   chan backend.Event
	events  chan backend.Event
	done    chan struct{}
	once    sync.Once
}

func (s *fakeSession) Handle(ctx context.Context, clientEvent backend.Event) error {
	var fixture struct {
		ServerType    string          `json:"fixture_server_type"`
		ServerPayload json.RawMessage `json:"fixture_server_payload"`
	}
	if json.Unmarshal(clientEvent.Data, &fixture) == nil && fixture.ServerType != "" && len(fixture.ServerPayload) > 0 {
		return s.send(ctx, event(fixture.ServerType, string(fixture.ServerPayload)))
	}
	if s.surface == backend.SessionRealtime {
		switch clientEvent.Type {
		case "session.update":
			return s.send(ctx, event("session.updated", `{"type":"session.updated","event_id":"event_updated","session":{"id":"sess_test","object":"realtime.session","type":"realtime"}}`))
		case "response.create":
			return s.send(ctx, event("response.done", `{"type":"response.done","event_id":"event_done","response":{"id":"resp_test","object":"realtime.response","status":"completed","status_details":null,"output":[]}}`))
		}
		return nil
	}
	if clientEvent.Type == "response.create" {
		if err := s.send(ctx, event("response.created", `{"type":"response.created","sequence_number":0,"response":{"id":"resp_test","object":"response","status":"in_progress","output":[]}}`)); err != nil {
			return err
		}
		return s.send(ctx, event("response.completed", `{"type":"response.completed","sequence_number":1,"response":{"id":"resp_test","object":"response","status":"completed","output":[]}}`))
	}
	return nil
}

func (s *fakeSession) send(ctx context.Context, value backend.Event) error {
	select {
	case s.queue <- value:
		return nil
	case <-s.done:
		return context.Canceled
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *fakeSession) run() {
	defer close(s.events)
	for {
		select {
		case value := <-s.queue:
			select {
			case s.events <- value:
			case <-s.done:
				return
			}
		case <-s.done:
			return
		}
	}
}

func (s *fakeSession) Events() <-chan backend.Event { return s.events }

func (s *fakeSession) Close(context.Context) error {
	s.once.Do(func() { close(s.done) })
	return nil
}

func event(eventType, data string) backend.Event {
	return backend.Event{Type: eventType, Data: json.RawMessage(data)}
}
