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
			"get":        {OperationID: "listModels", Tags: []string{"Models"}, Parameters: []parameter{{In: "query"}}, Responses: map[string]response{"200": {Content: map[string]json.RawMessage{"application/json": nil}}}},
			"parameters": {},
		},
		"/containers/{id}/content": {
			"post": {OperationID: "upload", RequestBody: &requestBody{Content: map[string]json.RawMessage{"application/octet-stream": nil}}, Responses: map[string]response{"201": {Content: map[string]json.RawMessage{"application/json": nil, "application/octet-stream": nil}}}},
		},
	}}
	operations := collectOperations(spec)
	if len(operations) != 2 || operations[0].OperationID != "upload" || operations[0].Capability != "containers" || operations[0].SuccessStatus != 201 || operations[0].SDKCall != "resource_helper" || operations[1].Capability != "models" || !operations[1].HasQuery {
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
	matrix := renderCompatibility([]generatedOperation{{OperationID: "listModels", Method: "GET", Path: "/models", Capability: "models", SDKCall: "resource_helper", LocalCase: "http.models.listModels", Transports: []string{"query", "json"}, SchemaCases: []string{"minimal_valid_request"}, LiveProfile: "safe", LiveReason: "scheduled"}}, eventInventory{
		Surfaces: map[string]eventSurface{"realtime": {Client: []string{"session.update"}, Server: []string{"session.updated"}}},
	})
	text := string(matrix)
	for _, required := range []string{`operation: "listModels"`, `type: "session.update"`, `local_semantic: validated_official_sdk`, `live_profile: safe`} {
		if !strings.Contains(text, required) {
			t.Fatalf("matrix missing %q:\n%s", required, text)
		}
	}
}

func TestEvidenceClassification(t *testing.T) {
	raw, helper := 0, 0
	for operation := range rawOnlyOperations {
		if operation == "" {
			t.Fatal("empty raw-only operation")
		}
		raw++
	}
	if raw != 8 {
		t.Fatalf("raw-only operations = %d", raw)
	}
	for _, operation := range []generatedOperation{
		{Method: "POST", Path: "/audio/transcriptions", RequestMedia: []string{"multipart/form-data"}, ResponseMedia: []string{"application/json", "text/event-stream"}},
		{Method: "GET", Path: "/files/{file_id}/content", ResponseMedia: []string{"application/octet-stream"}},
	} {
		if len(operationTransports(operation)) == 0 {
			t.Fatal("operation has no transport evidence")
		}
		helper++
	}
	if helper != 2 {
		t.Fatal("test setup did not exercise helper classifications")
	}
}

func TestValidateEvidenceRejectsMissingAndDuplicateMetadata(t *testing.T) {
	complete := make([]generatedOperation, 0, len(rawOnlyOperations))
	for operationID := range rawOnlyOperations {
		complete = append(complete, generatedOperation{
			OperationID: operationID, SDKCall: "raw_sdk_exception", LocalCase: "http." + operationID,
			Transports: []string{"json"}, SchemaCases: []string{"minimal"}, LiveProfile: "unavailable",
		})
	}
	if err := validateEvidence(complete, eventInventory{}); err != nil {
		t.Fatalf("complete evidence rejected: %v", err)
	}
	duplicate := append(append([]generatedOperation(nil), complete...), complete[0])
	if err := validateEvidence(duplicate, eventInventory{}); err == nil || !strings.Contains(err.Error(), "duplicate operation") {
		t.Fatalf("duplicate evidence error = %v", err)
	}
	missing := append([]generatedOperation(nil), complete[1:]...)
	if err := validateEvidence(missing, eventInventory{}); err == nil || !strings.Contains(err.Error(), "absent") {
		t.Fatalf("missing evidence error = %v", err)
	}
	badProfile := append([]generatedOperation(nil), complete...)
	badProfile[0].LiveProfile = "full"
	if err := validateEvidence(badProfile, eventInventory{}); err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("invalid profile error = %v", err)
	}
	events := eventInventory{Surfaces: map[string]eventSurface{"realtime": {Client: []string{"session.update", "session.update"}}}}
	if err := validateEvidence(complete, events); err == nil || !strings.Contains(err.Error(), "duplicate event") {
		t.Fatalf("duplicate event error = %v", err)
	}
}
