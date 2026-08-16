package aiservershell_test

import (
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

func TestServerRejectsDuplicatePattern(t *testing.T) {
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	if _, err := aiservershell.New(
		aiservershell.WithHandler("/", handler),
		aiservershell.WithHandler("/", handler),
	); err == nil {
		t.Fatal("duplicate pattern was accepted")
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
