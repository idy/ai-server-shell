# OpenAI M1 compatibility

The frozen profile is OpenAPI commit
`2186421dca0cca7c1e67caa7739005e8b1ccc4dd` (182 paths, 288 operations) and
official Node SDK `v7.4.0`. `spec/openai/upstream.lock` records the source digest;
`spec/openai/realtime-events.json` records 121 direction-specific event entries.

Credential-free local evidence exercises all 288 operations through the pinned
official SDK with request and response schema validation enabled. Of these, 280
use public resource helpers and eight machine-checked exceptions use the SDK's
raw request surface because `openai-node@7.4.0` exposes no helper. The backend
asserts the canonical operation and returns an operation-specific valid fixture,
which the SDK must parse.

The WebSocket suite covers all 121 direction-specific entries: client events
are sent through the Realtime or Responses SDK surface, while representative
server payloads are observed through SDK listeners. Discriminators and unknown
fields are preserved. The generated matrix records route, local semantic,
transport/schema, and live classifications independently; route inventory alone
is never reported as semantic compatibility.

OpenAPI 3.1 schemas are validated at the boundary. The runtime validator's
OpenAPI 3.0 representation rewrites `type: "null"` branches to equivalent
nullable schemas and permits the explicitly enumerated 3.1 keywords that the
validator version cannot interpret. The vendored source is never rewritten.

Requests are currently bounded and buffered up to the configured limit before
backend dispatch, including multipart and binary inputs. Binary responses stream
from `io.ReadCloser`; SSE and WebSockets flush incrementally. This is a declared
deviation from zero-copy multipart ingestion and should be considered when
choosing limits for large uploads.

Live differential profiles run the same named SDK case directly and through a
test-only public-interface backend. `safe` runs read-only `models.list`;
`paid` runs one bounded embedding request and requires an explicit cost gate;
`mutation` creates and deletes one batch-purpose file and requires cost,
mutation, and disposable-account approval. Every result is structured as
`PASS`, `SKIP`, or `FAIL`; the current cases emit `PASS` only after assertions,
and mutation success additionally requires verified deletion. Operations and
events outside these named live cases remain explicitly unverified in the
matrix.
