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
	session := &fakeSession{surface: request.Surface, events: make(chan backend.Event, 8)}
	if request.Surface == backend.SessionRealtime {
		session.events <- event("session.created", `{"type":"session.created","event_id":"event_test","session":{"id":"sess_test","object":"realtime.session","type":"realtime"}}`)
	}
	return session, nil
}

type fakeSession struct {
	mu      sync.Mutex
	surface backend.SessionSurface
	events  chan backend.Event
	closed  bool
}

func (s *fakeSession) Handle(_ context.Context, clientEvent backend.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return context.Canceled
	}
	var fixture struct {
		ServerType    string          `json:"fixture_server_type"`
		ServerPayload json.RawMessage `json:"fixture_server_payload"`
	}
	if json.Unmarshal(clientEvent.Data, &fixture) == nil && fixture.ServerType != "" && len(fixture.ServerPayload) > 0 {
		s.events <- event(fixture.ServerType, string(fixture.ServerPayload))
		return nil
	}
	if s.surface == backend.SessionRealtime {
		switch clientEvent.Type {
		case "session.update":
			s.events <- event("session.updated", `{"type":"session.updated","event_id":"event_updated","session":{"id":"sess_test","object":"realtime.session","type":"realtime"}}`)
		case "response.create":
			s.events <- event("response.done", `{"type":"response.done","event_id":"event_done","response":{"id":"resp_test","object":"realtime.response","status":"completed","status_details":null,"output":[]}}`)
		}
		return nil
	}
	if clientEvent.Type == "response.create" {
		s.events <- event("response.created", `{"type":"response.created","sequence_number":0,"response":{"id":"resp_test","object":"response","status":"in_progress","output":[]}}`)
		s.events <- event("response.completed", `{"type":"response.completed","sequence_number":1,"response":{"id":"resp_test","object":"response","status":"completed","output":[]}}`)
	}
	return nil
}

func (s *fakeSession) Events() <-chan backend.Event { return s.events }

func (s *fakeSession) Close(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		close(s.events)
	}
	return nil
}

func event(eventType, data string) backend.Event {
	return backend.Event{Type: eventType, Data: json.RawMessage(data)}
}
