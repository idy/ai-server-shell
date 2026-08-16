//go:build compatibility

package openai_compatibility

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/idy/ai-server-shell/backend"
)

type upstreamBackend struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func (u *upstreamBackend) Handle(ctx context.Context, request backend.Request) (backend.Response, error) {
	if request.Operation != "listModels" {
		return backend.Response{}, &backend.Error{Kind: backend.ErrorUnsupported, Message: "safe profile only enables listModels"}
	}
	upstreamRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, u.baseURL+"/models", bytes.NewReader(request.Input.Bytes))
	if err != nil {
		return backend.Response{}, err
	}
	upstreamRequest.Header.Set("Authorization", "Bearer "+u.apiKey)
	response, err := u.client.Do(upstreamRequest)
	if err != nil {
		return backend.Response{}, &backend.Error{Kind: backend.ErrorUnavailable, Message: "upstream request failed", Cause: err}
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	response.Body.Close()
	if err != nil {
		return backend.Response{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return backend.Response{}, fmt.Errorf("upstream returned HTTP %d", response.StatusCode)
	}
	return backend.Response{StatusCode: response.StatusCode, MediaType: "application/json", JSON: body}, nil
}

func newUpstreamBackend(baseURL, apiKey string) *upstreamBackend {
	return &upstreamBackend{baseURL: baseURL, apiKey: apiKey, client: &http.Client{Timeout: 30 * time.Second}}
}
