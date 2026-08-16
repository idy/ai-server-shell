package openai_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/idy/ai-server-shell/backend"
	"github.com/idy/ai-server-shell/openai"
)

type nilAuthenticator struct{}

func (*nilAuthenticator) Authenticate(context.Context, openai.Credential) (openai.Principal, error) {
	return openai.Principal{}, nil
}

func TestHandlerOptionsRejectInvalidValues(t *testing.T) {
	services, _ := backend.NewServices()
	var typedNil *nilAuthenticator
	tests := []struct {
		name   string
		option openai.Option
	}{
		{"nil option", nil},
		{"relative base", openai.WithBasePath("v1")},
		{"root base", openai.WithBasePath("/")},
		{"body limit", openai.WithMaxBodyBytes(0)},
		{"nil authenticator", openai.WithAuthenticator(typedNil)},
		{"empty origin", openai.WithOriginPatterns(" ")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := openai.NewHandler(services, test.option); err == nil {
				t.Fatal("invalid option was accepted")
			}
		})
	}
}

func TestCustomBasePathAndAuthentication(t *testing.T) {
	var credential openai.Credential
	services, _ := backend.NewServices(backend.WithModels(backend.HandlerFunc(func(context.Context, backend.Request) (backend.Response, error) {
		return backend.Response{JSON: []byte(`{"object":"list","data":[]}`)}, nil
	})))
	handler, err := openai.NewHandler(services,
		openai.WithBasePath("/openai/v1/"),
		openai.WithMaxBodyBytes(1024),
		openai.WithOriginPatterns("example.com"),
		openai.WithAuthenticator(openai.AuthenticatorFunc(func(_ context.Context, input openai.Credential) (openai.Principal, error) {
			credential = input
			if input.Token != "good" {
				return openai.Principal{}, errors.New("denied")
			}
			return openai.Principal{ID: "caller"}, nil
		})),
	)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/openai/v1/models", nil)
	request.Header.Set("Authorization", "Bearer good")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || credential.Token != "good" || credential.Source != "authorization" {
		t.Fatalf("status=%d credential=%#v", recorder.Code, credential)
	}

	request = httptest.NewRequest(http.MethodGet, "/openai/v1/models", nil)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", recorder.Code)
	}
}

func TestAuthenticatorRejectsEmptyPrincipal(t *testing.T) {
	services, _ := backend.NewServices(backend.WithModels(backend.HandlerFunc(func(context.Context, backend.Request) (backend.Response, error) {
		return backend.Response{}, nil
	})))
	handler, err := openai.NewHandler(services, openai.WithAuthenticator(openai.AuthenticatorFunc(func(context.Context, openai.Credential) (openai.Principal, error) {
		return openai.Principal{}, nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", recorder.Code)
	}
}
