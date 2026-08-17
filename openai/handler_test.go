package openai_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/idy/ai-server-shell/backend"
	"github.com/idy/ai-server-shell/openai"
	"github.com/idy/ai-server-shell/openai/internal/profile"
)

var pathParameter = regexp.MustCompile(`\{[^}]+\}`)

func TestHandlerDispatchesFrozenOperation(t *testing.T) {
	var received backend.Request
	services, err := backend.NewServices(backend.WithHandler(backend.HandlerFunc(func(_ context.Context, request backend.Request) (backend.Response, error) {
		received = request
		return backend.Response{JSON: []byte(`{"object":"list","data":[]}`)}, nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	handler, err := openai.NewHandler(services)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if received.Operation != "listModels" || received.Capability != backend.CapabilityModels {
		t.Fatalf("request = %#v", received)
	}
}

func TestHandlerRoutesEveryFrozenOperation(t *testing.T) {
	var received string
	services, err := backend.NewServices(backend.WithHandler(backend.HandlerFunc(func(_ context.Context, request backend.Request) (backend.Response, error) {
		received = request.Operation
		return backend.Response{JSON: []byte(`{}`)}, nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	handler, err := openai.NewHandler(services, openai.WithoutSchemaValidation())
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range profile.Operations {
		operation := operation
		t.Run(operation.ID, func(t *testing.T) {
			received = ""
			target := "/v1" + pathParameter.ReplaceAllString(operation.Path, "test-id")
			request := httptest.NewRequest(operation.Method, target, nil)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != operation.SuccessStatus {
				t.Fatalf("%s %s status = %d, want %d; body = %s", operation.Method, target, recorder.Code, operation.SuccessStatus, recorder.Body.String())
			}
			if received != operation.ID {
				t.Fatalf("%s %s dispatched %q, want %q", operation.Method, target, received, operation.ID)
			}
		})
	}
}

func TestHandlerReportsMissingCapability(t *testing.T) {
	services, err := backend.NewServices()
	if err != nil {
		t.Fatal(err)
	}
	handler, err := openai.NewHandler(services)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	if recorder.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandlerRejectsUnknownRoute(t *testing.T) {
	services, _ := backend.NewServices()
	handler, err := openai.NewHandler(services)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/not-real", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d", recorder.Code)
	}
}
