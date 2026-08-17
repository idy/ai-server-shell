package testutil

import "github.com/idy/ai-server-shell/backend"

// ScriptedStream is a deterministic, idempotently closable stream for SDK
// integration scenarios.
type ScriptedStream struct {
	events chan backend.Event
}

// NewScriptedStream returns a stream containing events in the supplied order.
func NewScriptedStream(events ...backend.Event) *ScriptedStream {
	stream := &ScriptedStream{events: make(chan backend.Event, len(events))}
	for _, event := range events {
		stream.events <- event
	}
	close(stream.events)
	return stream
}

func (s *ScriptedStream) Events() <-chan backend.Event { return s.events }

func (s *ScriptedStream) Close() error {
	return nil
}
