//go:build compatibility

package openai_compatibility

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"net/http/httptest"

	"github.com/idy/ai-server-shell/backend"
	openaihandler "github.com/idy/ai-server-shell/openai"
)

func TestLiveDifferential(t *testing.T) {
	profile := os.Getenv("OPENAI_COMPAT_PROFILE")
	if profile != "safe" && profile != "paid" && profile != "mutation" {
		t.Skip("set OPENAI_COMPAT_PROFILE=safe, paid, or mutation")
	}
	if profile != "safe" && os.Getenv("OPENAI_COMPAT_ALLOW_COST") != "1" {
		t.Fatal(profile + " profile requires OPENAI_COMPAT_ALLOW_COST=1")
	}
	if profile == "mutation" && os.Getenv("OPENAI_COMPAT_ALLOW_MUTATION") != "1" {
		t.Fatal("mutation profile requires OPENAI_COMPAT_ALLOW_MUTATION=1")
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

	direct := runSDK(t, runner, "https://api.openai.com/v1", upstreamKey, "direct")
	if err := validateEvidence(profile, direct); err != nil {
		t.Fatal(err)
	}
	upstream := newUpstreamBackend("https://api.openai.com/v1", upstreamKey)
	services, err := backend.NewServices(backend.WithHandler(upstream))
	if err != nil {
		t.Fatal(err)
	}
	handler, err := openaihandler.NewHandler(services)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	shell := runSDK(t, runner, server.URL+"/v1", "shell-test-credential", "shell")
	if err := validateEvidence(profile, shell); err != nil {
		t.Fatal(err)
	}
	if !observationsEqual(direct, shell) {
		t.Fatalf("normalized observations differ\ndirect=%#v\nshell=%#v", direct, shell)
	}
	for _, result := range shell.Cases {
		t.Logf("%s profile=%s case=%s target=differential cleanup=%s", result.Outcome, profile, result.Name, result.Cleanup.Outcome)
	}
}

type observation struct {
	Profile string            `json:"profile"`
	Target  string            `json:"target"`
	Cases   []caseObservation `json:"cases"`
}

type caseObservation struct {
	Name        string         `json:"name"`
	Outcome     string         `json:"outcome"`
	Observation map[string]any `json:"observation"`
	Cleanup     struct {
		Required bool   `json:"required"`
		Outcome  string `json:"outcome"`
	} `json:"cleanup"`
}

func runSDK(t *testing.T, runner, baseURL, apiKey, target string) observation {
	t.Helper()
	command := exec.Command("node", runner)
	command.Env = append(os.Environ(), "AI_SHELL_BASE_URL="+baseURL, "AI_SHELL_API_KEY="+apiKey, "OPENAI_COMPAT_TARGET="+target)
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
