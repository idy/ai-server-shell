package backend

import (
	"encoding/json"
	"io"
)

// Response is a protocol-neutral operation result. Exactly one of JSON, Body,
// or Stream should be set.
type Response struct {
	StatusCode int
	Metadata   map[string][]string
	MediaType  string
	JSON       json.RawMessage
	Body       io.ReadCloser
	Stream     Stream
}

// Event is one ordered streaming or session event.
type Event struct {
	Type string
	ID   string
	Data json.RawMessage
}

// Stream is application-owned until Close is called. Events must be closed by
// the producer. Close must be idempotent and unblock Events.
type Stream interface {
	Events() <-chan Event
	Close() error
}
