//go:build integration

package openai_integration_test

import (
	"encoding/json"
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
	spec := filepath.Join(repoRoot, "spec", "openai", "openapi.json")
	if _, err := os.Stat(filepath.Join(repoRoot, "tests", "sdk", "openai-node", "node_modules", "openai")); err != nil {
		t.Fatal("run npm --prefix tests/sdk/openai-node ci before the integration suite")
	}

	fake := &testutil.FakeBackend{}
	services, err := backend.NewServices(backend.WithHandler(fake))
	if err != nil {
		t.Fatal(err)
	}
	handler, err := openaihandler.NewHandler(services)
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
		"AI_SHELL_OPENAPI_SPEC="+spec,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("official SDK runner failed: %v\n%s", err, output)
	}
	var result struct {
		Expected      int      `json:"expected"`
		Passed        int      `json:"passed"`
		HelperCases   int      `json:"helper_cases"`
		RawCases      int      `json:"raw_cases"`
		Failures      []any    `json:"failures"`
		NegativeCases []string `json:"negative_cases"`
		StreamCases   []string `json:"stream_cases"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("decode SDK result: %v\n%s", err, output)
	}
	if result.Expected != 288 || result.Passed != result.Expected || result.HelperCases != 280 || result.RawCases != 8 || len(result.Failures) != 0 || len(result.NegativeCases) != 1 || len(result.StreamCases) != 1 {
		t.Fatalf("official SDK semantic cases: expected=%d passed=%d failures=%v", result.Expected, result.Passed, result.Failures)
	}
	if got := len(fake.Requests()); got != 289 {
		t.Fatalf("backend request count = %d, want 289", got)
	}
	seen := make(map[string]int, 288)
	media := make(map[string]bool)
	parameterized := 0
	for _, request := range fake.Requests() {
		seen[request.Operation]++
		media[request.Input.MediaType] = true
		if len(request.Parameters) > 0 {
			parameterized++
		}
		if request.Metadata.Protocol != "openai" || request.Metadata.CallerID != "anonymous" || request.Metadata.RequestID == "" {
			t.Fatalf("incomplete canonical metadata for %s: %#v", request.Operation, request.Metadata)
		}
		if request.Metadata.Extensions["Authorization"] != nil || request.Metadata.Extensions["Cookie"] != nil {
			t.Fatalf("unsafe credentials reached backend metadata for %s", request.Operation)
		}
		if len(request.Input.Bytes) > 0 && request.Input.MediaType == "" {
			t.Fatalf("request body for %s has no canonical media type", request.Operation)
		}
	}
	if len(seen) != 288 || parameterized == 0 || !media["application/json"] || !media["multipart/form-data"] || !media["application/sdp"] {
		t.Fatalf("canonical coverage operations=%d parameterized=%d media=%v", len(seen), parameterized, media)
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
		"AI_SHELL_EVENT_INVENTORY="+filepath.Join(repoRoot, "spec", "openai", "realtime-events.json"),
		"NODE_TLS_REJECT_UNAUTHORIZED=0",
		"NODE_NO_WARNINGS=1",
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("official SDK WebSocket runner failed: %v\n%s", err, output)
	}
	var result struct {
		Expected int      `json:"expected"`
		Passed   int      `json:"passed"`
		Covered  []string `json:"covered"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("decode SDK WebSocket result: %v\n%s", err, output)
	}
	if result.Expected != 121 || result.Passed != 121 || len(result.Covered) != 121 {
		t.Fatalf("unexpected SDK WebSocket result: %s", output)
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve integration test path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
}
