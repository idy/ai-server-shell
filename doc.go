// Package aiservershell provides a pluggable server-side shell for exposing
// OpenAI-compatible APIs from application-defined Go services.
//
// Protocol handlers are independently constructed from the shared,
// protocol-neutral backend.Services registry. The root Server is only an
// optional http.Handler aggregate; applications may serve a protocol handler
// directly.
package aiservershell
