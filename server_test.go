package aiservershell_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	aiservershell "github.com/idy/ai-server-shell"
)

func TestServerMountsIndependentHandlers(t *testing.T) {
	server, err := aiservershell.New(
		aiservershell.WithHandler("/openai/", http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusNoContent)
		})),
	)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/openai/v1/models", nil))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d", recorder.Code)
	}
}

type lifecycleHandler struct {
	http.Handler
	shutdown bool
}

func (h *lifecycleHandler) Shutdown(context.Context) error {
	h.shutdown = true
	return nil
}

func TestServerShutsDownMountedProtocolHandlers(t *testing.T) {
	handler := &lifecycleHandler{Handler: http.NotFoundHandler()}
	server, err := aiservershell.New(aiservershell.WithHandler("/", handler))
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !handler.shutdown {
		t.Fatal("mounted handler was not shut down")
	}
}

func TestServerRejectsDuplicatePattern(t *testing.T) {
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	if _, err := aiservershell.New(
		aiservershell.WithHandler("/", handler),
		aiservershell.WithHandler("/", handler),
	); err == nil {
		t.Fatal("duplicate pattern was accepted")
	}
}

func TestServerRejectsConflictingPatternsWithoutPanic(t *testing.T) {
	handler := http.NotFoundHandler()
	if _, err := aiservershell.New(
		aiservershell.WithHandler("/{first}", handler),
		aiservershell.WithHandler("/{second}", handler),
	); err == nil {
		t.Fatal("conflicting patterns were accepted")
	}
}

type nilHTTPHandler struct{}

func (*nilHTTPHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}

func TestServerRejectsInvalidHandlers(t *testing.T) {
	var typedNil *nilHTTPHandler
	for _, test := range []struct {
		name    string
		pattern string
		handler http.Handler
	}{
		{"empty pattern", "", http.NotFoundHandler()},
		{"invalid pattern", "GET [", http.NotFoundHandler()},
		{"typed nil", "/", typedNil},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := aiservershell.New(aiservershell.WithHandler(test.pattern, test.handler)); err == nil {
				t.Fatal("invalid handler was accepted")
			}
		})
	}
}
