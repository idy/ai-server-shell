package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/idy/ai-server-shell/backend"
)

func TestWriteSSEFramesInOrderAndCloses(t *testing.T) {
	stream := &testStream{events: make(chan backend.Event, 2)}
	stream.events <- backend.Event{Type: "response.output_text.delta", ID: "event\n1", Data: json.RawMessage(`{"delta":"hello"}`)}
	stream.events <- backend.Event{Type: "response.completed"}
	close(stream.events)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/", nil)
	writeSSE(request.Context(), recorder, "req_test", http.StatusCreated, stream)
	got := recorder.Body.String()
	if recorder.Code != http.StatusCreated || !strings.Contains(got, "id: event1\n") || !strings.Contains(got, "event: response.output_text.delta\n") || !strings.Contains(got, "data: {\"delta\":\"hello\"}\n\n") || !stream.closed {
		t.Fatalf("SSE=%q closed=%v", got, stream.closed)
	}
}

type testStream struct {
	events chan backend.Event
	closed bool
}

func (s *testStream) Events() <-chan backend.Event { return s.events }
func (s *testStream) Close() error                 { s.closed = true; return nil }
