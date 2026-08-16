# AI Server Shell

AI Server Shell is a Go framework for exposing application-defined AI behavior
through SDK-compatible server protocols. The shell owns routing, validation,
authentication hooks, HTTP/SSE/WebSocket lifecycle, and OpenAI-shaped errors;
your backend owns models, agents, tools, memory, persistence, and business logic.

Milestone 1 freezes the OpenAI handler to 182 paths and 288 operations from
`openai/openai-openapi` commit `2186421dca0cca7c1e67caa7739005e8b1ccc4dd`,
plus the Realtime and Responses WebSocket unions in official Node SDK `v7.4.0`.
Anthropic and Gemini handlers are future work and will consume the same
protocol-neutral `backend.Services` registry.

## Use it

Implement the public backend contract, inject one object for every capability
or selected objects for individual capabilities, then construct a normal
`http.Handler`:

```go
services, err := backend.NewServices(
    backend.WithHandler(app),
    backend.WithRealtime(app),
    backend.WithResponsesWebSocket(app),
)
if err != nil {
    log.Fatal(err)
}

handler, err := openai.NewHandler(services)
if err != nil {
    log.Fatal(err)
}
log.Fatal(http.ListenAndServe(":8080", handler))
```

`backend.Handler` receives a canonical capability, stable operation ID,
metadata, path/query parameters, and bounded JSON or opaque bytes. A missing
capability returns a stable OpenAI-shaped `501`; it is not silently simulated.
Bidirectional sessions implement `backend.SessionBackend` and `backend.Session`.

The optional root aggregate mounts already-constructed protocol handlers and
does not introduce another backend contract:

```go
shell, err := aiservershell.New(
    aiservershell.WithHandler("/v1/", handler),
)
```

The application owns the listener. During graceful shutdown, stop the
`http.Server` and call `shell.Shutdown(ctx)`; the latter prevents new protocol
upgrades, cancels active OpenAI WebSocket sessions, and waits for them to exit.
Unary HTTP and SSE calls follow their request contexts.

See [`examples/minimal`](examples/minimal/main.go),
[`docs/backend-contract.md`](docs/backend-contract.md), and
[`docs/openai-compatibility.md`](docs/openai-compatibility.md).

## Validate it

```sh
make verify

# Opt-in live, read-only differential check. Requires OPENAI_API_KEY.
OPENAI_COMPAT_PROFILE=safe make compatibility-safe
```

`make verify` regenerates the frozen profile, rejects drift, runs unit/race/vet,
installs the pinned official SDK, and drives it through real local listeners.
The local suite requires no network credential. Live tests keep the incoming
Shell credential separate from the upstream key and never forward it.

## Compatibility boundary

The committed route and event inventories are reproducible coverage inputs, not
a promise to adopt later upstream changes automatically. Unknown fields inside
known events are preserved. Unknown routes are 404; frozen routes with missing
backend capabilities are 501. Current limitations and live evidence are stated
in [`docs/openai-compatibility.md`](docs/openai-compatibility.md).

Licensed under the [Apache License 2.0](LICENSE).
