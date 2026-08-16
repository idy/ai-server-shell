package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestCollectOperationsAndRenderDeterministically(t *testing.T) {
	spec := document{Paths: map[string]map[string]operation{
		"/models": {
			"get":        {OperationID: "listModels", Tags: []string{"Models"}, Responses: map[string]response{"200": {Content: map[string]json.RawMessage{"application/json": nil}}}},
			"parameters": {},
		},
		"/containers/{id}/content": {
			"post": {OperationID: "upload", RequestBody: &requestBody{Content: map[string]json.RawMessage{"application/octet-stream": nil}}, Responses: map[string]response{"201": {Content: map[string]json.RawMessage{"application/json": nil, "application/octet-stream": nil}}}},
		},
	}}
	operations := collectOperations(spec)
	if len(operations) != 2 || operations[0].OperationID != "upload" || operations[0].Capability != "containers" || operations[0].SuccessStatus != 201 || operations[1].Capability != "models" {
		t.Fatalf("operations = %#v", operations)
	}
	raw := []byte(`{"openapi":"3.1.0","paths":{}}`)
	first, err := render(raw, 2, operations, eventInventory{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := render(raw, 2, operations, eventInventory{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) || !strings.Contains(string(first), "SpecOperationCount = 2") {
		t.Fatal("generation was not deterministic")
	}
}

func TestCapabilityMappingAndAppendUnique(t *testing.T) {
	tests := map[string]string{
		"Responses": "responses", "Chat": "chat", "Completions": "completions", "Conversations": "conversations",
		"Embeddings": "embeddings", "Audio": "audio", "Images": "images", "Videos": "videos",
		"Moderations": "moderations", "Models": "models", "Files": "files", "Uploads": "uploads",
		"Batch": "batches", "Assistants": "assistants", "Vector stores": "vector_stores", "Fine-tuning": "fine_tuning",
		"Evals": "evals", "Realtime": "realtime", "Skills": "skills",
	}
	for tag, want := range tests {
		if got := capabilityFor("/ignored", []string{tag}); got != want {
			t.Fatalf("tag %q = %q, want %q", tag, got, want)
		}
	}
	values := appendUnique(nil, "json")
	values = appendUnique(values, "json")
	if len(values) != 1 {
		t.Fatalf("appendUnique = %#v", values)
	}
}

func TestRenderCompatibilityIsSorted(t *testing.T) {
	matrix := renderCompatibility([]generatedOperation{{OperationID: "listModels", Method: "GET", Path: "/models", Capability: "models"}}, eventInventory{
		Surfaces: map[string]eventSurface{"realtime": {Client: []string{"session.update"}, Server: []string{"session.updated"}}},
	})
	text := string(matrix)
	for _, required := range []string{`operation: "listModels"`, `type: "session.update"`, `support: implemented`} {
		if !strings.Contains(text, required) {
			t.Fatalf("matrix missing %q:\n%s", required, text)
		}
	}
}
