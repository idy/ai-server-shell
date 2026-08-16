# Architecture

The dependency direction is deliberately one-way:

```text
OpenAI SDK -> openai.Handler -> backend.Services -> application backend
future SDK -> future handler -> backend.Services -> same application backend
```

The shell owns protocol parsing, route/schema validation, safe header mapping,
response/error formatting, bounded request bodies, streaming, cancellation,
WebSocket coordination, and connection cleanup. Applications own all semantics:
model execution, agents, tools, memory, persistence, authorization decisions,
and resource state.

`backend` imports no wire-protocol package. Values keep canonical operation and
capability identities, detached metadata, JSON documents, or opaque media bytes.
Protocol-specific fields remain in raw JSON or namespaced metadata so future
handlers can map them without making application implementations import an
OpenAI type.

`backend.Services` is validated and copied once. A default handler may serve all
unary capabilities, while explicit capability registrations override it. The
same concrete object can be registered repeatedly. Realtime and Responses
WebSocket sessions use separate injection slots because their lifecycle differs
from unary HTTP calls.

The aggregate `aiservershell.Server` only mounts independently constructed
`http.Handler` values. It does not own listeners or application backends.
