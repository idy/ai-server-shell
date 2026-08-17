//go:build compatibility

package openai_compatibility

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"net/http/httptest"

	"github.com/idy/ai-server-shell/backend"
	openaihandler "github.com/idy/ai-server-shell/openai"
)

func TestSafeLiveDifferential(t *testing.T) {
	if profile := os.Getenv("OPENAI_COMPAT_PROFILE"); profile != "safe" && profile != "full" {
		t.Skip("set OPENAI_COMPAT_PROFILE=safe or full")
	}
	if os.Getenv("OPENAI_COMPAT_PROFILE") == "full" {
		if os.Getenv("OPENAI_COMPAT_ALLOW_MUTATION") != "1" {
			t.Fatal("full profile requires OPENAI_COMPAT_ALLOW_MUTATION=1")
		}
		t.Fatal("full profile operation cases are not implemented; refusing to report a false compatibility pass")
	}
	upstreamKey := os.Getenv("OPENAI_API_KEY")
	if upstreamKey == "" {
		t.Fatal("OPENAI_API_KEY is required for live compatibility")
	}
	root := repositoryRoot(t)
	runner := filepath.Join(root, "tests", "sdk", "openai-node", "live.mjs")
	if _, err := os.Stat(filepath.Join(root, "tests", "sdk", "openai-node", "node_modules", "openai")); err != nil {
		t.Fatal("run npm --prefix tests/sdk/openai-node ci first")
	}

	direct := runSDK(t, runner, "https://api.openai.com/v1", upstreamKey)
	services, err := backend.NewServices(backend.WithModels(newUpstreamBackend("https://api.openai.com/v1", upstreamKey)))
	if err != nil {
		t.Fatal(err)
	}
	handler, err := openaihandler.NewHandler(services)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	shell := runSDK(t, runner, server.URL+"/v1", "shell-test-credential")
	if !reflect.DeepEqual(direct, shell) {
		t.Fatalf("normalized observations differ\ndirect=%s\nshell=%s", direct, shell)
	}
	t.Logf("PASS profile=%s operation=listModels target=local cleanup=not-required models=%d", os.Getenv("OPENAI_COMPAT_PROFILE"), len(direct.Data))
}

type observation struct {
	Operation string `json:"operation"`
	Object    string `json:"object"`
	Data      []struct {
		ID     string `json:"id"`
		Object string `json:"object"`
	} `json:"data"`
}

func runSDK(t *testing.T, runner, baseURL, apiKey string) observation {
	t.Helper()
	command := exec.Command("node", runner)
	command.Env = append(os.Environ(), "AI_SHELL_BASE_URL="+baseURL, "AI_SHELL_API_KEY="+apiKey)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("official SDK live runner failed: %v\n%s", err, output)
	}
	var result observation
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("decode live observation: %v\n%s", err, output)
	}
	return result
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}
