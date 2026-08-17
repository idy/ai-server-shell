package rest

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"reflect"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/idy/ai-server-shell/backend"
	"github.com/idy/ai-server-shell/openai/internal/contract"
	"github.com/idy/ai-server-shell/openai/internal/profile"
)

type handler struct {
	services         backend.Services
	router           routers.Router
	validationRoutes map[string]*routers.Route
	config           contract.Config
}

func New(services backend.Services, document *openapi3.T, router routers.Router, config contract.Config) http.Handler {
	return &handler{
		services: services, router: router,
		validationRoutes: validationRoutesByOperationID(document), config: config,
	}
}

func (h *handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	requestID := request.Header.Get("X-Request-Id")
	if requestID == "" {
		requestID = newRequestID()
	}
	writer.Header().Set("X-Request-Id", requestID)

	callerID, err := h.config.Authenticate(request.Context(), request, requestID)
	if err != nil {
		writeError(writer, requestID, &backend.Error{Kind: backend.ErrorUnauthorized, Message: "Invalid authentication credentials."})
		return
	}

	limited := io.LimitReader(request.Body, h.config.MaxBodyBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		writeError(writer, requestID, &backend.Error{Kind: backend.ErrorInvalid, Message: "Could not read request body."})
		return
	}
	if int64(len(body)) > h.config.MaxBodyBytes {
		writeErrorStatus(writer, requestID, http.StatusRequestEntityTooLarge, &backend.Error{Kind: backend.ErrorInvalid, Message: "Request body is too large."})
		return
	}
	request.Body = io.NopCloser(bytes.NewReader(body))

	route, pathParams, err := h.router.FindRoute(request)
	if err != nil {
		writeErrorStatus(writer, requestID, routeErrorStatus(err), &backend.Error{Kind: backend.ErrorNotFound, Message: "The requested resource was not found."})
		return
	}
	operation, ok := profile.OperationForRoute(request.Method, route.Path, request.URL.Query())
	if !ok {
		operation, ok = profile.OperationByID(route.Operation.OperationID)
	}
	if !ok {
		writeErrorStatus(writer, requestID, http.StatusNotImplemented, &backend.Error{Kind: backend.ErrorUnsupported, Message: "Operation is not in the active compatibility profile."})
		return
	}
	validationRoute := h.validationRoute(route, operation)
	validationInput := &openapi3filter.RequestValidationInput{
		Request: request, PathParams: pathParams, QueryParams: request.URL.Query(), Route: validationRoute,
		Options: &openapi3filter.Options{
			AuthenticationFunc:                openapi3filter.NoopAuthenticationFunc,
			MultiError:                        true,
			RejectWhenRequestBodyNotSpecified: validationRoute.Operation.RequestBody == nil,
			// The backend receives the original wire payload, so applying schema
			// defaults to a decoded copy has no effect. It also makes kin-openapi
			// attempt to re-encode multipart bodies, which it does not support.
			SkipSettingDefaults: true,
		},
	}
	if h.config.Validate {
		request.Body = io.NopCloser(bytes.NewReader(body))
		if err := openapi3filter.ValidateRequest(request.Context(), validationInput); err != nil {
			writeError(writer, requestID, &backend.Error{Kind: backend.ErrorInvalid, Message: safeValidationMessage(err)})
			return
		}
	}

	handler, ok := h.services.HandlerFor(backend.Capability(operation.Capability))
	if !ok {
		writeErrorStatus(writer, requestID, http.StatusNotImplemented, &backend.Error{
			Kind: backend.ErrorUnsupported, Code: "capability_not_configured",
			Message: fmt.Sprintf("The %s capability is not configured.", operation.Capability),
		})
		return
	}

	result, err := handler.Handle(request.Context(), backend.Request{
		Capability: backend.Capability(operation.Capability),
		Operation:  operation.ID,
		Metadata: backend.Metadata{
			RequestID: requestID, CallerID: callerID, Protocol: "openai",
			Extensions: safeHeaders(request.Header),
		},
		Parameters: parameters(pathParams, request),
		Input:      input(body, request.Header.Get("Content-Type")),
	})
	if err != nil {
		writeBackendError(writer, requestID, err)
		return
	}
	defer closeResponse(result)
	if responseModeCount(result) > 1 {
		if nonNilInterface(result.Stream) {
			_ = result.Stream.Close()
		}
		writeErrorStatus(writer, requestID, http.StatusInternalServerError, &backend.Error{Kind: backend.ErrorInternal, Message: "Backend returned an ambiguous response."})
		return
	}

	status := result.StatusCode
	if status == 0 {
		status = operation.SuccessStatus
	}
	if status < 100 || status > 599 {
		writeErrorStatus(writer, requestID, http.StatusInternalServerError, &backend.Error{Kind: backend.ErrorInternal, Message: "Backend returned an invalid HTTP status."})
		return
	}
	copySafeHeaders(writer.Header(), result.Metadata)
	if nonNilInterface(result.Stream) {
		writeSSE(request.Context(), writer, requestID, status, result.Stream)
		return
	}
	if nonNilInterface(result.Body) {
		if result.MediaType != "" {
			writer.Header().Set("Content-Type", result.MediaType)
		}
		writer.WriteHeader(status)
		_, _ = io.Copy(writer, result.Body)
		return
	}
	mediaType := result.MediaType
	if mediaType == "" {
		mediaType = "application/json"
	}
	writer.Header().Set("Content-Type", mediaType)
	if h.config.Validate && len(result.JSON) > 0 && strings.Contains(mediaType, "json") {
		responseInput := &openapi3filter.ResponseValidationInput{
			RequestValidationInput: validationInput, Status: status, Header: writer.Header().Clone(),
			Options: &openapi3filter.Options{IncludeResponseStatus: true, MultiError: true},
		}
		responseInput.SetBodyBytes(result.JSON)
		if err := openapi3filter.ValidateResponse(request.Context(), responseInput); err != nil {
			writeErrorStatus(writer, requestID, http.StatusInternalServerError, &backend.Error{Kind: backend.ErrorInternal, Message: "Backend returned a response outside the active compatibility profile."})
			return
		}
	}
	writer.WriteHeader(status)
	if len(result.JSON) > 0 {
		_, _ = writer.Write(result.JSON)
	}
}

func (h *handler) validationRoute(matched *routers.Route, operation profile.Operation) *routers.Route {
	indexed, ok := h.validationRoutes[operation.ID]
	if !ok {
		return matched
	}
	selected := *matched
	selected.Path = indexed.Path
	selected.PathItem = indexed.PathItem
	selected.Operation = indexed.Operation
	return &selected
}

func validationRoutesByOperationID(document *openapi3.T) map[string]*routers.Route {
	routes := make(map[string]*routers.Route)
	var server *openapi3.Server
	if len(document.Servers) > 0 {
		server = document.Servers[0]
	}
	for profilePath, pathItem := range document.Paths.Map() {
		canonicalPath, _, _ := strings.Cut(profilePath, "?")
		for method, operation := range pathItem.Operations() {
			if operation.OperationID == "" {
				continue
			}
			routes[operation.OperationID] = &routers.Route{
				Spec: document, Server: server, Path: canonicalPath, PathItem: pathItem,
				Method: method, Operation: operation,
			}
		}
	}
	return routes
}

func input(body []byte, contentType string) backend.Input {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = contentType
	}
	result := backend.Input{MediaType: mediaType, Bytes: append([]byte(nil), body...)}
	if strings.Contains(mediaType, "json") && len(body) > 0 {
		result.JSON = append(json.RawMessage(nil), body...)
	}
	return result
}

func parameters(pathParams map[string]string, request *http.Request) map[string]json.RawMessage {
	result := make(map[string]json.RawMessage, len(pathParams)+len(request.URL.Query()))
	for key, value := range pathParams {
		encoded, _ := json.Marshal(value)
		result[key] = encoded
	}
	for key, values := range request.URL.Query() {
		var value any = values
		if len(values) == 1 {
			value = values[0]
		}
		encoded, _ := json.Marshal(value)
		result[key] = encoded
	}
	return result
}

func safeHeaders(headers http.Header) map[string][]string {
	result := make(map[string][]string)
	for key, values := range headers {
		if strings.EqualFold(key, "Authorization") || strings.EqualFold(key, "Cookie") || strings.EqualFold(key, "Sec-WebSocket-Protocol") {
			continue
		}
		result[http.CanonicalHeaderKey(key)] = append([]string(nil), values...)
	}
	return result
}

func copySafeHeaders(target http.Header, source map[string][]string) {
	for key, values := range source {
		if hopByHop(key) || strings.EqualFold(key, "Authorization") || strings.EqualFold(key, "Set-Cookie") {
			continue
		}
		for _, value := range values {
			target.Add(key, value)
		}
	}
}

func hopByHop(header string) bool {
	switch http.CanonicalHeaderKey(header) {
	case "Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade":
		return true
	default:
		return false
	}
}

func closeResponse(response backend.Response) {
	if nonNilInterface(response.Body) {
		_ = response.Body.Close()
	}
}

func responseModeCount(response backend.Response) int {
	count := 0
	if len(response.JSON) > 0 {
		count++
	}
	if nonNilInterface(response.Body) {
		count++
	}
	if nonNilInterface(response.Stream) {
		count++
	}
	return count
}

func nonNilInterface(value any) bool {
	if value == nil {
		return false
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return !reflected.IsNil()
	default:
		return true
	}
}

func routeErrorStatus(err error) int {
	if errors.Is(err, routers.ErrMethodNotAllowed) {
		return http.StatusMethodNotAllowed
	}
	return http.StatusNotFound
}

func newRequestID() string {
	var value [12]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "req_unknown"
	}
	return "req_" + hex.EncodeToString(value[:])
}

func safeValidationMessage(err error) string {
	message := strings.ReplaceAll(err.Error(), "\n", " ")
	if len(message) > 500 {
		message = message[:500]
	}
	return "Invalid request: " + message
}
