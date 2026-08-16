// Package realtime defines application-facing contracts for the OpenAI
// Realtime-compatible server shell.
package realtime

import (
	"context"
	"encoding/json"
)

// Service creates an application-owned session for an accepted Realtime
// connection.
//
// This is an initial design contract and may change before the first release.
type Service interface {
	Open(context.Context, OpenRequest) (Session, error)
}

// Session receives client events and exposes a stream of server events.
// Implementations own domain behavior; the shell owns wire encoding, transport
// lifecycle, protocol validation, and error mapping.
type Session interface {
	Handle(context.Context, ClientEvent) error
	Events() <-chan ServerEvent
	Close(context.Context) error
}

// OpenRequest contains connection metadata resolved by the shell before a
// session implementation is created.
type OpenRequest struct {
	SessionID string
	Model     string
	Metadata  map[string]string
}

// ClientEvent is the forward-compatible envelope delivered to a Session.
// Typed event variants will be added as the M0 protocol profile is specified.
type ClientEvent struct {
	ID      string
	Type    string
	Payload json.RawMessage
}

// ServerEvent is the forward-compatible envelope emitted by a Session.
// The shell is responsible for validating and serializing it on the wire.
type ServerEvent struct {
	ID      string
	Type    string
	Payload json.RawMessage
}
