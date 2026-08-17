# OpenAI M1 compatibility

The frozen profile is OpenAPI commit
`2186421dca0cca7c1e67caa7739005e8b1ccc4dd` (182 paths, 288 operations) and
official Node SDK `v7.4.0`. `spec/openai/upstream.lock` records the source digest;
`spec/openai/realtime-events.json` records 121 direction-specific event entries.

Local evidence exercises all 288 generated operation routes through the pinned
official SDK raw request surface plus a generated `models.list` helper. Separate
official SDK cases establish Realtime and Responses WebSocket connections over
TLS and observe `session.updated` and `response.completed`. Unit coverage checks
that every frozen event discriminator round-trips while preserving unknown JSON.

OpenAPI 3.1 schemas are validated at the boundary. The runtime validator's
OpenAPI 3.0 representation rewrites `type: "null"` branches to equivalent
nullable schemas and permits the explicitly enumerated 3.1 keywords that the
validator version cannot interpret. The vendored source is never rewritten.

Requests are currently bounded and buffered up to the configured limit before
backend dispatch, including multipart and binary inputs. Binary responses stream
from `io.ReadCloser`; SSE and WebSockets flush incrementally. This is a declared
deviation from zero-copy multipart ingestion and should be considered when
choosing limits for large uploads.

The safe live profile is read-only and compares official SDK `models.list`
observations directly and through a test-only public-interface backend. It costs
no inference and creates no resource. Other live surfaces remain unverified in
M1's full profile; they must not be described as live-compatible until a
disposable account, explicit mutation/cost approval, case assertions, and
cleanup evidence exist.
