// Package openai provides the OpenAI-compatible protocol handler.
package openai

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/idy/ai-server-shell/backend"
	"github.com/idy/ai-server-shell/openai/internal/contract"
	"github.com/idy/ai-server-shell/openai/internal/profile"
	"github.com/idy/ai-server-shell/openai/internal/rest"
	ws "github.com/idy/ai-server-shell/openai/internal/websocket"
)

// Handler implements the frozen OpenAI M1 protocol profile.
type Handler struct {
	basePath  string
	rest      http.Handler
	websocket http.Handler
}

// NewHandler constructs an independently mountable OpenAI http.Handler.
func NewHandler(services backend.Services, options ...Option) (*Handler, error) {
	settings := settings{basePath: "/v1", maxBodyBytes: defaultMaxBodyBytes, validate: true}
	for _, option := range options {
		if option == nil {
			return nil, fmt.Errorf("openai: nil option")
		}
		if err := option(&settings); err != nil {
			return nil, err
		}
	}

	specJSON, err := profile.RouterSpecJSON()
	if err != nil {
		return nil, err
	}
	loader := openapi3.NewLoader()
	document, err := loader.LoadFromData(specJSON)
	if err != nil {
		return nil, fmt.Errorf("openai: load frozen profile: %w", err)
	}
	document.Servers = openapi3.Servers{&openapi3.Server{URL: settings.basePath}}
	router, err := legacy.NewRouter(
		document,
		openapi3.AllowExtraSiblingFields(
			"$recursiveRef", "$recursiveAnchor", "propertyNames", "max_items", "min_items", "optional", "const", "identifier", "webhooks",
		),
		openapi3.DisableExamplesValidation(),
		openapi3.DisableSchemaDefaultsValidation(),
	)
	if err != nil {
		return nil, fmt.Errorf("openai: build frozen profile router: %w", err)
	}

	internalConfig := contract.Config{
		BasePath: settings.basePath, MaxBodyBytes: settings.maxBodyBytes,
		OriginPatterns: append([]string(nil), settings.originPatterns...), Validate: settings.validate,
	}
	internalConfig.Authenticate = func(ctx context.Context, request *http.Request, requestID string) (string, error) {
		if settings.authenticator == nil {
			return "anonymous", nil
		}
		token, source := credentialFromRequest(request)
		principal, err := settings.authenticator.Authenticate(ctx, Credential{
			Token: token, Source: source, RequestID: requestID,
		})
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(principal.ID) == "" {
			return "", fmt.Errorf("openai: authenticator returned an empty principal")
		}
		return principal.ID, nil
	}

	return &Handler{
		basePath:  settings.basePath,
		rest:      rest.New(services, document, router, internalConfig),
		websocket: ws.New(services, internalConfig),
	}, nil
}

func (h *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if !strings.HasPrefix(request.URL.Path, h.basePath+"/") && request.URL.Path != h.basePath {
		http.NotFound(writer, request)
		return
	}
	if isWebSocket(request) && (request.URL.Path == h.basePath+"/realtime" || request.URL.Path == h.basePath+"/responses") {
		h.websocket.ServeHTTP(writer, request)
		return
	}
	h.rest.ServeHTTP(writer, request)
}

func isWebSocket(request *http.Request) bool {
	return strings.EqualFold(request.Header.Get("Upgrade"), "websocket") &&
		strings.Contains(strings.ToLower(request.Header.Get("Connection")), "upgrade")
}

func credentialFromRequest(request *http.Request) (string, string) {
	if authorization := request.Header.Get("Authorization"); strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
		return strings.TrimSpace(authorization[len("Bearer "):]), "authorization"
	}
	for _, protocol := range request.Header.Values("Sec-WebSocket-Protocol") {
		for _, candidate := range strings.Split(protocol, ",") {
			candidate = strings.TrimSpace(candidate)
			const prefix = "openai-insecure-api-key."
			if strings.HasPrefix(candidate, prefix) {
				return strings.TrimPrefix(candidate, prefix), "websocket_subprotocol"
			}
		}
	}
	return "", "none"
}
