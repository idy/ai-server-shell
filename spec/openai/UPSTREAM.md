# OpenAI M1 upstream profile

Milestone 1 is frozen to `openai/openai-openapi` commit
`2186421dca0cca7c1e67caa7739005e8b1ccc4dd` and official OpenAI Node SDK
`v7.4.0`. The vendored OpenAPI document contains 182 paths and 288 HTTP
operations.

Run `go generate ./...` after an explicitly reviewed profile update. Generation
must be deterministic. A newer upstream document is drift evidence, not an
automatic compatibility claim.

The vendored OpenAPI document retains its upstream MIT license. Realtime and
Responses WebSocket event inventories are separately pinned to the SDK tag
because those bidirectional protocols are not fully described by OpenAPI.
