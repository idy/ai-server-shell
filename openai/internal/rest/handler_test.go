package rest

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/idy/ai-server-shell/backend"
	"github.com/idy/ai-server-shell/openai/internal/contract"
	"github.com/idy/ai-server-shell/openai/internal/profile"
)

func testHandler(t *testing.T, services backend.Services, validate bool, authenticate contract.Authenticate) http.Handler {
	t.Helper()
	data, err := profile.RouterSpecJSON()
	if err != nil {
		t.Fatal(err)
	}
	document, err := openapi3.NewLoader().LoadFromData(data)
	if err != nil {
		t.Fatal(err)
	}
	document.Servers = openapi3.Servers{&openapi3.Server{URL: "/v1"}}
	router, err := legacy.NewRouter(document,
		openapi3.AllowExtraSiblingFields("$recursiveRef", "$recursiveAnchor", "propertyNames", "max_items", "min_items", "optional", "const", "identifier", "webhooks"),
		openapi3.DisableExamplesValidation(), openapi3.DisableSchemaDefaultsValidation())
	if err != nil {
		t.Fatal(err)
	}
	if authenticate == nil {
		authenticate = func(context.Context, *http.Request, string) (string, error) { return "caller", nil }
	}
	return New(services, document, router, contract.Config{BasePath: "/v1", MaxBodyBytes: 32, Validate: validate, Authenticate: authenticate})
}

func TestHandlerMapsRequestAndResponse(t *testing.T) {
	var received backend.Request
	services, _ := backend.NewServices(backend.WithModels(backend.HandlerFunc(func(_ context.Context, request backend.Request) (backend.Response, error) {
		received = request
		return backend.Response{JSON: []byte(`{"object":"list","data":[]}`), Metadata: map[string][]string{"X-Test": {"yes"}, "Set-Cookie": {"no"}}}, nil
	})))
	handler := testHandler(t, services, true, nil)
	request := httptest.NewRequest(http.MethodGet, "/v1/models?after=cursor", nil)
	request.Header.Set("Authorization", "Bearer secret")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Header().Get("X-Test") != "yes" || recorder.Header().Get("Set-Cookie") != "" {
		t.Fatalf("response = %d %#v %s", recorder.Code, recorder.Header(), recorder.Body.String())
	}
	if received.Operation != "listModels" || received.Metadata.CallerID != "caller" || received.Metadata.Extensions["Authorization"] != nil || string(received.Parameters["after"]) != `"cursor"` {
		t.Fatalf("backend request = %#v", received)
	}
}

func TestHandlerFailureBoundaries(t *testing.T) {
	services, _ := backend.NewServices()
	tests := []struct {
		name    string
		handler http.Handler
		request *http.Request
		status  int
	}{
		{"too large", testHandler(t, services, false, nil), httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(make([]byte, 33))), http.StatusRequestEntityTooLarge},
		{"unknown route", testHandler(t, services, false, nil), httptest.NewRequest(http.MethodGet, "/v1/missing", nil), http.StatusNotFound},
		{"missing capability", testHandler(t, services, false, nil), httptest.NewRequest(http.MethodGet, "/v1/models", nil), http.StatusNotImplemented},
		{"auth", testHandler(t, services, false, func(context.Context, *http.Request, string) (string, error) { return "", errors.New("denied") }), httptest.NewRequest(http.MethodGet, "/v1/models", nil), http.StatusUnauthorized},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			test.handler.ServeHTTP(recorder, test.request)
			if recorder.Code != test.status {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, test.status, recorder.Body.String())
			}
		})
	}
}

func TestValidationRouteSelectsBetaSchema(t *testing.T) {
	services, _ := backend.NewServices()
	h := testHandler(t, services, true, nil).(*handler)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses?beta=true", bytes.NewReader([]byte(`{}`)))
	request.Header.Set("Content-Type", "application/json")
	matched, _, err := h.router.FindRoute(request)
	if err != nil {
		t.Fatal(err)
	}
	operation, ok := profile.OperationForRoute(request.Method, matched.Path, request.URL.Query())
	if !ok {
		t.Fatal("beta operation was not selected")
	}
	selected := h.validationRoute(matched, operation)
	if selected.Operation.OperationID != "beta_createResponse" || selected.Path != "/responses" {
		t.Fatalf("validation route = %s %s", selected.Path, selected.Operation.OperationID)
	}
}

func TestHandlerMapsBackendErrors(t *testing.T) {
	tests := []struct {
		kind   backend.ErrorKind
		status int
	}{
		{backend.ErrorInvalid, 400}, {backend.ErrorUnauthorized, 401}, {backend.ErrorForbidden, 403},
		{backend.ErrorNotFound, 404}, {backend.ErrorConflict, 409}, {backend.ErrorRateLimit, 429},
		{backend.ErrorUnsupported, 501}, {backend.ErrorTimeout, 408}, {backend.ErrorCanceled, 499},
		{backend.ErrorUnavailable, 503}, {backend.ErrorInternal, 500},
	}
	for _, test := range tests {
		t.Run(string(test.kind), func(t *testing.T) {
			services, _ := backend.NewServices(backend.WithModels(backend.HandlerFunc(func(context.Context, backend.Request) (backend.Response, error) {
				return backend.Response{}, &backend.Error{Kind: test.kind, Message: "safe", RetryAfter: 3}
			})))
			recorder := httptest.NewRecorder()
			testHandler(t, services, false, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
			if recorder.Code != test.status || recorder.Header().Get("Retry-After") != "3" || !bytes.Contains(recorder.Body.Bytes(), []byte(`"message":"safe"`)) {
				t.Fatalf("response = %d %#v %s", recorder.Code, recorder.Header(), recorder.Body.String())
			}
		})
	}
}

func TestHandlerStreamsBinaryAndClosesIt(t *testing.T) {
	body := &recordingReadCloser{Reader: bytes.NewBufferString("binary")}
	services, _ := backend.NewServices(backend.WithModels(backend.HandlerFunc(func(context.Context, backend.Request) (backend.Response, error) {
		return backend.Response{Body: body, MediaType: "application/octet-stream"}, nil
	})))
	recorder := httptest.NewRecorder()
	testHandler(t, services, false, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	if recorder.Body.String() != "binary" || !body.closed {
		t.Fatalf("body=%q closed=%v", recorder.Body.String(), body.closed)
	}
}

func TestHandlerRejectsInvalidBackendResponses(t *testing.T) {
	tests := []struct {
		name     string
		response backend.Response
	}{
		{"ambiguous", backend.Response{JSON: []byte(`{}`), Body: io.NopCloser(bytes.NewBufferString("body"))}},
		{"invalid status", backend.Response{StatusCode: 700, JSON: []byte(`{}`)}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			services, _ := backend.NewServices(backend.WithModels(backend.HandlerFunc(func(context.Context, backend.Request) (backend.Response, error) {
				return test.response, nil
			})))
			recorder := httptest.NewRecorder()
			testHandler(t, services, false, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
			if recorder.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

type recordingReadCloser struct {
	io.Reader
	closed bool
}

func (r *recordingReadCloser) Close() error { r.closed = true; return nil }
