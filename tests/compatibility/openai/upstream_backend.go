//go:build compatibility

package openai_compatibility

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/idy/ai-server-shell/backend"
)

type upstreamBackend struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func (u *upstreamBackend) Handle(ctx context.Context, request backend.Request) (backend.Response, error) {
	method, path, ok := upstreamRoute(request)
	if !ok {
		return backend.Response{}, &backend.Error{Kind: backend.ErrorUnsupported, Message: "operation is not enabled by a live compatibility profile"}
	}
	upstreamRequest, err := http.NewRequestWithContext(ctx, method, u.baseURL+path, bytes.NewReader(request.Input.Bytes))
	if err != nil {
		return backend.Response{}, err
	}
	upstreamRequest.Header.Set("Authorization", "Bearer "+u.apiKey)
	if request.Input.MediaType != "" {
		contentType := request.Input.MediaType
		if values := request.Metadata.Extensions["Content-Type"]; len(values) > 0 {
			contentType = values[0]
		}
		upstreamRequest.Header.Set("Content-Type", contentType)
	}
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

func upstreamRoute(request backend.Request) (string, string, bool) {
	switch request.Operation {
	case "listModels":
		return http.MethodGet, "/models", true
	case "createEmbedding":
		return http.MethodPost, "/embeddings", true
	case "createFile":
		return http.MethodPost, "/files", true
	case "deleteFile":
		var id string
		if json.Unmarshal(request.Parameters["file_id"], &id) != nil || id == "" {
			return "", "", false
		}
		return http.MethodDelete, "/files/" + url.PathEscape(id), true
	default:
		return "", "", false
	}
}

func newUpstreamBackend(baseURL, apiKey string) *upstreamBackend {
	return &upstreamBackend{baseURL: baseURL, apiKey: apiKey, client: &http.Client{Timeout: 30 * time.Second}}
}
