# Backend contract

`backend.Handler.Handle` receives an immutable-by-contract `backend.Request`.
`Capability` selects a resource family and `Operation` is the frozen semantic
operation ID. `Metadata` carries request/caller identity, source protocol, and
redacted headers. `Parameters` preserves path and query values; `Input` owns the
bounded raw body and also exposes JSON when applicable.

A response selects JSON, an owned binary body, or an ordered stream. The shell
closes bodies and streams after serving them. Producers must close event
channels and make `Close` idempotent. Context cancellation means work should
stop promptly; implementations must not retain input buffers past the call.

Use `backend.Error` for safe failure mapping. Unknown errors become a generic
500 without serializing Go errors. Invalid, authentication, permission, missing,
conflict, rate-limit, unsupported, and unavailable failures have stable HTTP
mappings. `RetryAfter` is emitted only as a safe integer header.

Known WebSocket events preserve their complete raw JSON, including unknown
fields. A `Session` has one input call path and one output channel. `Close` must
unblock `Events` and remain safe under repeated/concurrent calls.

Registration fails for nil options, typed nil implementations, duplicate slots,
and unknown capability values. After construction, registry maps are private
copies and safe for concurrent reads.
