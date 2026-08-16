# Architecture

## Purpose

AI Server Shell is a protocol framework for implementing OpenAI-compatible servers in Go. It is deliberately positioned between client SDKs and application-defined behavior.

It is the reverse of a client SDK:

- A client SDK converts method calls into protocol messages.
- AI Server Shell converts protocol messages into application service calls.

## Ownership boundary

### The shell owns

- HTTP routes and transport negotiation.
- WebSocket, SSE, and future WebRTC protocol adapters.
- Authentication integration points.
- Request and event decoding.
- Protocol validation and error serialization.
- Connection and session lifecycle.
- Event ordering and identifier generation where required by the protocol.
- Cancellation propagation.
- Bounded queues, backpressure, and shutdown behavior.
- Compatibility fixtures and client-SDK conformance tests.

### An application owns

- Agent orchestration.
- Model inference or model gateway calls.
- Prompts and instructions.
- Function and tool execution.
- Conversation persistence and long-term memory.
- Domain data and authorization decisions.
- Safety policy and product behavior.
- The semantic content of emitted responses.

The shell must not import or depend on application packages.

## Service model

API surfaces are registered independently. Applications implement only the services they need.

```go
type Services struct {
	Realtime  realtime.Service
	Responses responses.Service
	Chat      chat.Service
}
```

An unregistered service does not expose its routes.

## Realtime session model

The Realtime service creates one application session per accepted protocol session.

```go
type Service interface {
	Open(context.Context, OpenRequest) (Session, error)
}

type Session interface {
	Handle(context.Context, ClientEvent) error
	Events() <-chan ServerEvent
	Close(context.Context) error
}
```

The transport reader decodes client events and calls `Handle`. A dedicated writer drains `Events`, validates server events, and writes them to the client. Cancellation of the connection context must stop both directions.

## Protocol profiles

OpenAI APIs evolve. The shell should define explicit, versioned protocol profiles rather than using an unqualified `compatible` boolean.

Each profile records:

- supported routes;
- supported client events;
- supported server events;
- accepted legacy aliases;
- unknown-field behavior;
- tested SDK names and versions;
- documented deviations.

## Extension policy

Applications may attach namespaced metadata and extension events when a negotiated profile allows them. Project-owned extensions must not collide with OpenAI event names. Unknown fields should be preserved when safe so newer clients do not fail solely because the shell has not promoted a field into a typed Go structure.

## Reliability principles

1. Every connection has a bounded memory budget.
2. Slow consumers create explicit backpressure or a documented close condition.
3. Cancellation is response-scoped whenever the wire protocol carries a response identifier.
4. Terminal events are emitted at most once per response.
5. Session close is idempotent.
6. Protocol errors do not panic the server.
7. No compatibility claim is accepted without a black-box client test.
