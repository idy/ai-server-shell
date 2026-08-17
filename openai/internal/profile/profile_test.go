package profile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"testing"
)

func TestFrozenProfileInventory(t *testing.T) {
	if len(Operations) != SpecOperationCount || SpecOperationCount != 288 {
		t.Fatalf("operation count = %d", len(Operations))
	}
	seen := make(map[string]bool, len(Operations))
	ids := make(map[string]bool, len(Operations))
	for _, operation := range Operations {
		key := operation.Method + " " + operation.Path
		if seen[key] {
			t.Fatalf("duplicate operation %s", key)
		}
		seen[key] = true
		if ids[operation.ID] {
			t.Fatalf("duplicate operation ID %s", operation.ID)
		}
		ids[operation.ID] = true
		if operation.ID == "" || operation.Capability == "" {
			t.Fatalf("incomplete operation %#v", operation)
		}
	}
	raw, err := SpecJSON()
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	if got := hex.EncodeToString(digest[:]); got != SpecSHA256 {
		t.Fatalf("spec digest = %s", got)
	}
}

func TestRouterProfileAndLookups(t *testing.T) {
	raw, err := RouterSpecJSON()
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil || document["paths"] == nil {
		t.Fatalf("router document is invalid: %v", err)
	}
	operation, ok := OperationByID("listModels")
	if !ok || operation.Path != "/models" {
		t.Fatalf("listModels = %#v, %v", operation, ok)
	}
	if _, ok := OperationByID("missing"); ok {
		t.Fatal("unknown operation was found")
	}
	operation, ok = OperationForRoute("POST", "/responses", url.Values{"beta": {"true"}})
	if !ok || operation.ID != "beta_createResponse" {
		t.Fatalf("beta response = %#v, %v", operation, ok)
	}
	operation, ok = OperationForRoute("POST", "/responses", nil)
	if !ok || operation.ID != "createResponse" {
		t.Fatalf("response fallback = %#v, %v", operation, ok)
	}
	if _, ok := OperationForRoute("GET", "/missing", nil); ok {
		t.Fatal("unknown route was found")
	}
}

func TestSpecJSONReturnsIndependentCopies(t *testing.T) {
	first, err := SpecJSON()
	if err != nil {
		t.Fatal(err)
	}
	first[0] = 'x'
	second, err := SpecJSON()
	if err != nil {
		t.Fatal(err)
	}
	if second[0] == 'x' {
		t.Fatal("SpecJSON returned shared storage")
	}
}
