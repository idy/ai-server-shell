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
