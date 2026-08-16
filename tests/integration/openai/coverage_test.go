//go:build integration

package openai_integration_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCompatibilityMatrixCoversFrozenProfile(t *testing.T) {
	root := repositoryRoot(t)
	var manifest []struct {
		OperationID string `json:"OperationID"`
	}
	readJSON(t, filepath.Join(root, "tests", "sdk", "openai-node", "operations.json"), &manifest)
	var events struct {
		Surfaces map[string]struct {
			Client []string `json:"client"`
			Server []string `json:"server"`
		} `json:"surfaces"`
	}
	readJSON(t, filepath.Join(root, "spec", "openai", "realtime-events.json"), &events)
	data, err := os.ReadFile(filepath.Join(root, "spec", "openai", "compatibility.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var matrix struct {
		HTTP []struct {
			Operation string `yaml:"operation"`
			Support   string `yaml:"support"`
			LocalCase string `yaml:"local_case"`
		} `yaml:"http"`
		Events []struct {
			Surface   string `yaml:"surface"`
			Direction string `yaml:"direction"`
			Type      string `yaml:"type"`
			Support   string `yaml:"support"`
			LocalCase string `yaml:"local_case"`
		} `yaml:"events"`
	}
	if err := yaml.Unmarshal(data, &matrix); err != nil {
		t.Fatal(err)
	}
	if len(manifest) != 288 || len(matrix.HTTP) != len(manifest) {
		t.Fatalf("HTTP manifest=%d matrix=%d", len(manifest), len(matrix.HTTP))
	}
	for index, operation := range manifest {
		entry := matrix.HTTP[index]
		if entry.Operation != operation.OperationID || entry.Support != "implemented" || entry.LocalCase == "" {
			t.Fatalf("HTTP entry %d = %#v for %#v", index, entry, operation)
		}
	}
	wantEvents := map[string]bool{}
	for surface, inventory := range events.Surfaces {
		for _, eventType := range inventory.Client {
			wantEvents[surface+"/client/"+eventType] = true
		}
		for _, eventType := range inventory.Server {
			wantEvents[surface+"/server/"+eventType] = true
		}
	}
	if len(wantEvents) != 121 || len(matrix.Events) != len(wantEvents) {
		t.Fatalf("event inventory=%d matrix=%d", len(wantEvents), len(matrix.Events))
	}
	for _, entry := range matrix.Events {
		key := entry.Surface + "/" + entry.Direction + "/" + entry.Type
		if !wantEvents[key] || entry.Support != "implemented" || entry.LocalCase == "" {
			t.Fatalf("invalid event matrix entry %#v", entry)
		}
		delete(wantEvents, key)
	}
	if len(wantEvents) != 0 {
		t.Fatalf("uncovered events: %#v", wantEvents)
	}
}

func readJSON(t *testing.T, path string, output any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, output); err != nil {
		t.Fatal(err)
	}
}
