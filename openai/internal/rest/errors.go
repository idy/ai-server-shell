package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/idy/ai-server-shell/backend"
)

type errorEnvelope struct {
	Error errorObject `json:"error"`
}

type errorObject struct {
	Message string            `json:"message"`
	Type    backend.ErrorKind `json:"type"`
	Param   *string           `json:"param"`
	Code    *string           `json:"code"`
}

func writeBackendError(writer http.ResponseWriter, requestID string, err error) {
	var backendError *backend.Error
	if !errors.As(err, &backendError) {
		backendError = &backend.Error{Kind: backend.ErrorInternal, Message: "The server encountered an internal error."}
	}
	writeError(writer, requestID, backendError)
}

func writeError(writer http.ResponseWriter, requestID string, backendError *backend.Error) {
	writeErrorStatus(writer, requestID, errorStatus(backendError), backendError)
}

func writeErrorStatus(writer http.ResponseWriter, requestID string, status int, backendError *backend.Error) {
	if backendError.Kind == "" {
		backendError.Kind = backend.ErrorInternal
	}
	if backendError.Message == "" {
		backendError.Message = "The request could not be completed."
	}
	var code, param *string
	if backendError.Code != "" {
		code = &backendError.Code
	}
	if backendError.Param != "" {
		param = &backendError.Param
	}
	if backendError.RetryAfter > 0 {
		writer.Header().Set("Retry-After", strconv.Itoa(backendError.RetryAfter))
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("X-Request-Id", requestID)
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(errorEnvelope{Error: errorObject{
		Message: backendError.Message, Type: backendError.Kind, Param: param, Code: code,
	}})
}

func errorStatus(err *backend.Error) int {
	switch err.Kind {
	case backend.ErrorInvalid:
		return http.StatusBadRequest
	case backend.ErrorUnauthorized:
		return http.StatusUnauthorized
	case backend.ErrorForbidden:
		return http.StatusForbidden
	case backend.ErrorNotFound:
		return http.StatusNotFound
	case backend.ErrorConflict:
		return http.StatusConflict
	case backend.ErrorRateLimit:
		return http.StatusTooManyRequests
	case backend.ErrorUnsupported:
		return http.StatusNotImplemented
	case backend.ErrorTimeout:
		return http.StatusRequestTimeout
	case backend.ErrorCanceled:
		return 499
	case backend.ErrorUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
