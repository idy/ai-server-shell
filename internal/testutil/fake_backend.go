package testutil

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/idy/ai-server-shell/backend"
)

// FakeBackend records canonical requests and returns deterministic fixtures.
type FakeBackend struct {
	mu       sync.Mutex
	requests []backend.Request
}

func (f *FakeBackend) Handle(_ context.Context, request backend.Request) (backend.Response, error) {
	f.mu.Lock()
	f.requests = append(f.requests, request)
	f.mu.Unlock()
	if firstHeader(request, "X-Ai-Shell-Stream-Case") == "responses.basic" && request.Operation == "createResponse" {
		return backend.Response{StatusCode: 200, Stream: NewScriptedStream(
			backend.Event{Type: "response.created", Data: json.RawMessage(`{"type":"response.created","sequence_number":0,"response":{"id":"resp_stream","object":"response","status":"in_progress","output":[]}}`)},
			backend.Event{Type: "response.output_text.delta", Data: json.RawMessage(`{"type":"response.output_text.delta","sequence_number":1,"response_id":"resp_stream","item_id":"item_stream","output_index":0,"content_index":0,"delta":"hello"}`)},
			backend.Event{Type: "response.completed", Data: json.RawMessage(`{"type":"response.completed","sequence_number":2,"response":{"id":"resp_stream","object":"response","status":"completed","output":[]}}`)},
		)}, nil
	}
	if response, found, err := ResponseFixture(request); found {
		return response, err
	}
	if request.Operation == "listModels" {
		return backend.Response{JSON: json.RawMessage(`{"object":"list","data":[]}`)}, nil
	}
	return backend.Response{JSON: json.RawMessage(`{}`)}, nil
}

func (f *FakeBackend) Requests() []backend.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]backend.Request(nil), f.requests...)
}
