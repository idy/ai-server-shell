package testutil

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/idy/ai-server-shell/backend"
)

// FakeBackend records canonical requests and returns deterministic fixtures.
type FakeBackend struct {
	mu       sync.Mutex
	requests []backend.Request
	fixtures map[string]SemanticFixture
	errors   []error
}

// NewFakeBackend validates and installs an independent fixture for every case.
func NewFakeBackend(fixtures []SemanticFixture) (*FakeBackend, error) {
	result := &FakeBackend{fixtures: make(map[string]SemanticFixture, len(fixtures))}
	for _, fixture := range fixtures {
		if fixture.Operation == "" || result.fixtures[fixture.Operation].Operation != "" {
			return nil, fmt.Errorf("missing or duplicate semantic fixture %q", fixture.Operation)
		}
		result.fixtures[fixture.Operation] = fixture
	}
	return result, nil
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
	caseName := firstHeader(request, caseHeader)
	fixture, found := f.fixtures[caseName]
	if !found {
		return backend.Response{}, f.recordError(fmt.Errorf("unknown SDK semantic case %q", caseName))
	}
	if err := fixture.assertRequest(request); err != nil {
		return backend.Response{}, f.recordError(err)
	}
	response, err := fixture.response()
	if err != nil {
		return backend.Response{}, f.recordError(err)
	}
	return response, nil
}

func (f *FakeBackend) recordError(err error) error {
	f.mu.Lock()
	f.errors = append(f.errors, err)
	f.mu.Unlock()
	return err
}

func (f *FakeBackend) Requests() []backend.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]backend.Request(nil), f.requests...)
}

func (f *FakeBackend) Errors() []error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]error(nil), f.errors...)
}
