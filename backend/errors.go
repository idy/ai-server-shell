package backend

import "fmt"

// ErrorKind is a stable failure class translated by protocol handlers.
type ErrorKind string

const (
	ErrorInvalid      ErrorKind = "invalid_request_error"
	ErrorUnauthorized ErrorKind = "authentication_error"
	ErrorForbidden    ErrorKind = "permission_error"
	ErrorNotFound     ErrorKind = "not_found_error"
	ErrorConflict     ErrorKind = "conflict_error"
	ErrorRateLimit    ErrorKind = "rate_limit_error"
	ErrorUnsupported  ErrorKind = "unsupported_error"
	ErrorTimeout      ErrorKind = "timeout_error"
	ErrorCanceled     ErrorKind = "canceled_error"
	ErrorUnavailable  ErrorKind = "service_unavailable_error"
	ErrorInternal     ErrorKind = "server_error"
)

// Error describes a safe application failure. Cause is never serialized.
type Error struct {
	Kind       ErrorKind
	Code       string
	Param      string
	Message    string
	RetryAfter int
	Cause      error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("backend %s", e.Kind)
}

func (e *Error) Unwrap() error { return e.Cause }
