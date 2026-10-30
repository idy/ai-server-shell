package profile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"reflect"
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

func TestRouterSpecPromotesCommonAnyOfType(t *testing.T) {
	value := map[string]any{
		"anyOf": []any{
			map[string]any{"type": "string"},
			map[string]any{"type": "string", "enum": []any{"whisper-1"}},
		},
	}
	normalizeRouterSchemas(value, nativeOpenAPI31Supported)
	if value["type"] != "string" {
		t.Fatalf("promoted type = %#v", value["type"])
	}

	mixed := map[string]any{
		"anyOf": []any{
			map[string]any{"type": "string"},
			map[string]any{"type": "array"},
		},
	}
	normalizeRouterSchemas(mixed, nativeOpenAPI31Supported)
	if _, exists := mixed["type"]; exists {
		t.Fatalf("mixed union acquired type: %#v", mixed)
	}
}

func TestRouterSpecRequiredFieldsAreUnique(t *testing.T) {
	raw, err := RouterSpecJSON()
	if err != nil {
		t.Fatal(err)
	}
	var document any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	var check func(any)
	check = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			if required, ok := typed["required"].([]any); ok {
				seen := make(map[string]bool, len(required))
				for _, item := range required {
					name, ok := item.(string)
					if !ok || seen[name] {
						t.Fatalf("invalid required names in router document: %#v", required)
					}
					seen[name] = true
				}
			}
			for _, child := range typed {
				check(child)
			}
		case []any:
			for _, child := range typed {
				check(child)
			}
		}
	}
	check(document)
}

func TestRouterNormalizationPreservesRequiredOrderAndInvalidEntries(t *testing.T) {
	invalid := map[string]any{"invalid": true}
	value := map[string]any{
		"required": []any{"id", "name", "id", invalid, "name", invalid},
	}
	normalizeRouterSchemas(value, nativeOpenAPI31Supported)
	want := []any{"id", "name", invalid, invalid}
	if !reflect.DeepEqual(value["required"], want) {
		t.Fatalf("required = %#v, want %#v", value["required"], want)
	}
}

func TestRouterNormalizationPreservesSchemaConstraints(t *testing.T) {
	for _, tc := range []struct {
		name   string
		native bool
		input  map[string]any
		want   map[string]any
	}{
		{
			"legacy lower bound", false,
			map[string]any{"minimum": float64(0), "exclusiveMinimum": true},
			map[string]any{"minimum": float64(0), "exclusiveMinimum": true},
		},
		{
			"numeric lower bound", true,
			map[string]any{"minimum": float64(0), "exclusiveMinimum": true},
			map[string]any{"exclusiveMinimum": float64(0)},
		},
		{
			"numeric upper bound", true,
			map[string]any{"maximum": float64(1), "exclusiveMaximum": true},
			map[string]any{"exclusiveMaximum": float64(1)},
		},
		{
			"inclusive bound", true,
			map[string]any{"minimum": float64(0), "exclusiveMinimum": false},
			map[string]any{"minimum": float64(0)},
		},
		{
			"already numeric", true,
			map[string]any{"exclusiveMinimum": float64(2)},
			map[string]any{"exclusiveMinimum": float64(2)},
		},
		{
			"invalid missing bound", true,
			map[string]any{"exclusiveMinimum": true},
			map[string]any{"exclusiveMinimum": true},
		},
		{
			"legacy null", false,
			map[string]any{"type": "null"},
			map[string]any{"nullable": true, "enum": []any{nil}},
		},
		{
			"native null", true,
			map[string]any{"type": "null"},
			map[string]any{"type": "null"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			normalizeRouterSchemas(tc.input, tc.native)
			if !reflect.DeepEqual(tc.input, tc.want) {
				t.Fatalf("schema = %#v, want %#v", tc.input, tc.want)
			}
		})
	}
}
