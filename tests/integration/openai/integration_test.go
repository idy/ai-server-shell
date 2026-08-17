//go:build integration

package openai_integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"net/http/httptest"

	"github.com/idy/ai-server-shell/backend"
	"github.com/idy/ai-server-shell/internal/testutil"
	openaihandler "github.com/idy/ai-server-shell/openai"
)

func TestOfficialNodeSDKFrozenOperationInventory(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Fatal("node is required for the OpenAI SDK integration suite")
	}
	repoRoot := repositoryRoot(t)
	runner := filepath.Join(repoRoot, "tests", "sdk", "openai-node", "runner.mjs")
	manifest := filepath.Join(repoRoot, "tests", "sdk", "openai-node", "operations.json")
	if _, err := os.Stat(filepath.Join(repoRoot, "tests", "sdk", "openai-node", "node_modules", "openai")); err != nil {
		t.Fatal("run npm --prefix tests/sdk/openai-node ci before the integration suite")
	}

	fake := &testutil.FakeBackend{}
	services, err := backend.NewServices(backend.WithHandler(fake))
	if err != nil {
		t.Fatal(err)
	}
	handler, err := openaihandler.NewHandler(services, openaihandler.WithoutSchemaValidation())
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()

	command := exec.Command("node", runner)
	command.Env = append(os.Environ(),
		"AI_SHELL_BASE_URL="+server.URL+"/v1",
		"AI_SHELL_API_KEY=integration-test-key",
		"AI_SHELL_OPERATION_MANIFEST="+manifest,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("official SDK runner failed: %v\n%s", err, output)
	}
	var result struct {
		Expected int   `json:"expected"`
		Passed   int   `json:"passed"`
		Failures []any `json:"failures"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("decode SDK result: %v\n%s", err, output)
	}
	if result.Expected != 289 || result.Passed != result.Expected || len(result.Failures) != 0 {
		t.Fatalf("unexpected SDK result: %s", output)
	}
	if got := len(fake.Requests()); got != 289 {
		t.Fatalf("backend request count = %d, want 289", got)
	}
}

func TestOfficialNodeSDKWebSockets(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Fatal("node is required for the OpenAI SDK integration suite")
	}
	repoRoot := repositoryRoot(t)
	if _, err := os.Stat(filepath.Join(repoRoot, "tests", "sdk", "openai-node", "node_modules", "openai")); err != nil {
		t.Fatal("run npm --prefix tests/sdk/openai-node ci before the integration suite")
	}
	sessions := &testutil.FakeSessionBackend{}
	services, err := backend.NewServices(
		backend.WithSession(backend.SessionRealtime, sessions),
		backend.WithSession(backend.SessionResponsesSocket, sessions),
	)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := openaihandler.NewHandler(services)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(handler)
	defer server.Close()

	command := exec.Command("node", filepath.Join(repoRoot, "tests", "sdk", "openai-node", "websocket.mjs"))
	command.Env = append(os.Environ(),
		"AI_SHELL_BASE_URL="+server.URL+"/v1",
		"AI_SHELL_API_KEY=integration-test-key",
		"NODE_TLS_REJECT_UNAUTHORIZED=0",
		"NODE_NO_WARNINGS=1",
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("official SDK WebSocket runner failed: %v\n%s", err, output)
	}
	var result map[string]string
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("decode SDK WebSocket result: %v\n%s", err, output)
	}
	if result["realtime"] != "session.updated" || result["responses"] != "response.completed" {
		t.Fatalf("unexpected SDK WebSocket result: %s", output)
	}
}

func TestOfficialNodeSDKValidationBoundary(t *testing.T) {
	repoRoot := repositoryRoot(t)
	if _, err := os.Stat(filepath.Join(repoRoot, "tests", "sdk", "openai-node", "node_modules", "openai")); err != nil {
		t.Fatal("run npm --prefix tests/sdk/openai-node ci before the integration suite")
	}
	validated := &validationBackend{}
	services, err := backend.NewServices(backend.WithHandler(validated))
	if err != nil {
		t.Fatal(err)
	}
	handler, err := openaihandler.NewHandler(services)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	command := exec.Command("node", filepath.Join(repoRoot, "tests", "sdk", "openai-node", "validated.mjs"))
	command.Env = append(os.Environ(), "AI_SHELL_BASE_URL="+server.URL+"/v1", "AI_SHELL_API_KEY=integration-test-key")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("validation-enabled SDK runner failed: %v\n%s", err, output)
	}
	var result struct {
		Passed          int  `json:"passed"`
		InvalidRejected bool `json:"invalidRejected"`
	}
	if err := json.Unmarshal(output, &result); err != nil || result.Passed != 4 || !result.InvalidRejected {
		t.Fatalf("validation result=%s err=%v", output, err)
	}
	requests := validated.Requests()
	if len(requests) != 4 || requests[1].Input.MediaType != "application/json" || requests[2].Input.MediaType != "multipart/form-data" {
		t.Fatalf("validated backend requests = %#v", requests)
	}
}

type validationBackend struct {
	requests []backend.Request
}

func (b *validationBackend) Handle(_ context.Context, request backend.Request) (backend.Response, error) {
	b.requests = append(b.requests, request)
	switch request.Operation {
	case "listModels":
		return backend.Response{JSON: json.RawMessage(`{"object":"list","data":[],"first_id":null,"last_id":null,"has_more":false}`)}, nil
	case "createEmbedding":
		return backend.Response{JSON: json.RawMessage(`{"object":"list","model":"text-embedding-test","data":[{"object":"embedding","index":0,"embedding":[0.1]}],"usage":{"prompt_tokens":1,"total_tokens":1}}`)}, nil
	case "createFile":
		return backend.Response{JSON: json.RawMessage(`{"id":"file_test","object":"file","bytes":14,"created_at":1,"filename":"input.jsonl","purpose":"batch","status":"processed"}`)}, nil
	case "downloadFile":
		return backend.Response{MediaType: "application/octet-stream", Body: io.NopCloser(bytes.NewBufferString("binary-test"))}, nil
	default:
		return backend.Response{}, &backend.Error{Kind: backend.ErrorUnsupported, Message: "unexpected validation test operation"}
	}
}

func (b *validationBackend) Requests() []backend.Request {
	return append([]backend.Request(nil), b.requests...)
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve integration test path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
}
