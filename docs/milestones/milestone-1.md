# Milestone 1: frozen OpenAI server profile

M1 delivers the shared backend registry, standalone OpenAI handler, optional
aggregate server, frozen route/event generation, local official-SDK tests, and
an opt-in live differential harness. The source of truth is Issue #1 and the
committed files under `spec/openai`.

No later OpenAI change is adopted implicitly. Anthropic, Gemini, WebRTC media,
and application business semantics are outside this milestone.

The M1 closeout evidence separates frozen route inventory from local semantic
and live differential evidence. The credential-free gate covers 288 HTTP
operations and 121 direction-specific WebSocket entries with
`openai-node@7.4.0`; live evidence remains limited to the named safe, paid, and
mutation profiles documented in `docs/openai-compatibility.md`.
