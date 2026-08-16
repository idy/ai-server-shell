package backend

import "encoding/json"

// Request is the canonical unary or streaming operation delivered to a
// capability backend. Operation is a stable semantic action from the frozen
// compatibility profile; protocol route and header details stay in Metadata.
type Request struct {
	Capability Capability
	Operation  string
	Metadata   Metadata
	Parameters map[string]json.RawMessage
	Input      Input
}

// Input owns the decoded JSON document or bounded opaque bytes supplied by the
// protocol handler. JSON is populated for JSON-compatible media types. Bytes
// preserves multipart, audio, image, video, and other opaque request bodies.
type Input struct {
	MediaType string
	JSON      json.RawMessage
	Bytes     []byte
}
