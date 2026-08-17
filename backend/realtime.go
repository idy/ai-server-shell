package backend

import "context"

// SessionSurface identifies a bidirectional protocol-neutral session family.
type SessionSurface string

const (
	SessionRealtime        SessionSurface = "realtime"
	SessionResponsesSocket SessionSurface = "responses_websocket"
)

// SessionRequest contains immutable metadata resolved before a session opens.
type SessionRequest struct {
	Surface    SessionSurface
	Metadata   Metadata
	Parameters map[string]string
}

// SessionBackend creates application-owned bidirectional sessions.
type SessionBackend interface {
	OpenSession(context.Context, SessionRequest) (Session, error)
}

// Session receives client events and produces server events. Close must be
// idempotent and must unblock Events.
type Session interface {
	Handle(context.Context, Event) error
	Events() <-chan Event
	Close(context.Context) error
}
