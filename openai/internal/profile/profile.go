package profile

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
)

// Operation describes one method/path pair in the frozen M1 profile.
type Operation struct {
	Method        string
	Path          string
	ID            string
	Capability    string
	RequestMedia  []string
	ResponseMedia []string
	SuccessStatus int
}

// Event describes one direction-specific WebSocket event discriminator.
type Event struct {
	Surface   string
	Direction string
	Type      string
}

var (
	specOnce       sync.Once
	specJSON       []byte
	specErr        error
	operationOnce  sync.Once
	operationsByID map[string]Operation
	eventOnce      sync.Once
	eventSet       map[string]bool
)

// Probe decoding support instead of tying router normalization to a module version.
var nativeOpenAPI31Supported = func() bool {
	var schema openapi3.Schema
	return json.Unmarshal([]byte(`{"type":"null","exclusiveMinimum":0}`), &schema) == nil
}()

// SpecJSON returns an independent copy of the frozen OpenAPI document.
func SpecJSON() ([]byte, error) {
	specOnce.Do(func() {
		compressed, err := base64.StdEncoding.DecodeString(compressedSpecBase64)
		if err != nil {
			specErr = fmt.Errorf("decode embedded OpenAPI document: %w", err)
			return
		}
		reader, err := gzip.NewReader(bytes.NewReader(compressed))
		if err != nil {
			specErr = fmt.Errorf("open embedded OpenAPI document: %w", err)
			return
		}
		defer reader.Close()
		specJSON, specErr = io.ReadAll(reader)
		if specErr != nil {
			specErr = fmt.Errorf("read embedded OpenAPI document: %w", specErr)
		}
	})
	return append([]byte(nil), specJSON...), specErr
}

// EventAllowed reports whether a discriminator belongs to a frozen SDK event
// union for the given surface and direction.
func EventAllowed(surface, direction, eventType string) bool {
	eventOnce.Do(func() {
		eventSet = make(map[string]bool, len(Events))
		for _, event := range Events {
			eventSet[event.Surface+"\x00"+event.Direction+"\x00"+event.Type] = true
		}
	})
	return eventSet[surface+"\x00"+direction+"\x00"+eventType]
}

// RouterSpecJSON returns a semantically equivalent document accepted by both
// legacy and native OpenAPI 3.1 validators. The vendored source and its digest
// remain untouched.
func RouterSpecJSON() ([]byte, error) {
	raw, err := SpecJSON()
	if err != nil {
		return nil, err
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("decode frozen OpenAPI document: %w", err)
	}
	normalizeRouterSchemas(value, nativeOpenAPI31Supported)
	result, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode router OpenAPI document: %w", err)
	}
	return result, nil
}

func normalizeRouterSchemas(value any, nativeOpenAPI31 bool) {
	switch typed := value.(type) {
	case map[string]any:
		// Required names form a set. Normalize only the router copy so strict
		// validators accept redundant declarations in the frozen source.
		if required, ok := typed["required"].([]any); ok {
			seen := make(map[string]bool, len(required))
			unique := required[:0]
			for _, item := range required {
				name, isName := item.(string)
				if isName && seen[name] {
					continue
				}
				unique = append(unique, item)
				if isName {
					seen[name] = true
				}
			}
			typed["required"] = unique
		}
		if nativeOpenAPI31 {
			// Newer validators require the numeric OpenAPI 3.1 spelling.
			for _, bound := range [][2]string{
				{"minimum", "exclusiveMinimum"},
				{"maximum", "exclusiveMaximum"},
			} {
				if exclusive, ok := typed[bound[1]].(bool); ok {
					if !exclusive {
						delete(typed, bound[1])
					} else if value, ok := typed[bound[0]].(float64); ok {
						typed[bound[1]] = value
						delete(typed, bound[0])
					}
				}
			}
		}
		if schemaType, ok := typed["type"].(string); ok && schemaType == "null" && !nativeOpenAPI31 {
			delete(typed, "type")
			typed["nullable"] = true
			if _, exists := typed["enum"]; !exists {
				typed["enum"] = []any{nil}
			}
		}
		// kin-openapi v0.135 uses the parent type while decoding primitive
		// multipart fields. OpenAPI 3.1 permits that type to live only on the
		// anyOf branches, as the frozen OpenAI schema does for fields such as
		// audio model. Copying a type shared by every branch to the parent is
		// semantically equivalent and lets the decoder preserve the field value
		// for the subsequent schema validation pass.
		if _, exists := typed["type"]; !exists {
			if commonType, ok := commonAnyOfType(typed["anyOf"]); ok {
				typed["type"] = commonType
			}
		}
		for _, child := range typed {
			normalizeRouterSchemas(child, nativeOpenAPI31)
		}
	case []any:
		for _, child := range typed {
			normalizeRouterSchemas(child, nativeOpenAPI31)
		}
	}
}

func commonAnyOfType(value any) (string, bool) {
	branches, ok := value.([]any)
	if !ok || len(branches) == 0 {
		return "", false
	}
	var common string
	for _, branch := range branches {
		object, ok := branch.(map[string]any)
		if !ok {
			return "", false
		}
		branchType, ok := object["type"].(string)
		if !ok || branchType == "null" {
			return "", false
		}
		if common == "" {
			common = branchType
		} else if common != branchType {
			return "", false
		}
	}
	return common, true
}

// OperationByID returns one frozen operation descriptor.
func OperationByID(id string) (Operation, bool) {
	operationOnce.Do(func() {
		operationsByID = make(map[string]Operation, len(Operations))
		for _, operation := range Operations {
			operationsByID[operation.ID] = operation
		}
	})
	operation, ok := operationsByID[id]
	return operation, ok
}

// OperationForRoute resolves query-selected profile variants such as the
// frozen Responses beta operations after the OpenAPI router matched the path.
func OperationForRoute(method, routePath string, query url.Values) (Operation, bool) {
	var fallback Operation
	var foundFallback bool
	for _, operation := range Operations {
		operationPath, rawQuery, _ := strings.Cut(operation.Path, "?")
		if operation.Method != method || operationPath != routePath {
			continue
		}
		if rawQuery == "" {
			fallback, foundFallback = operation, true
			continue
		}
		required, err := url.ParseQuery(rawQuery)
		if err != nil {
			continue
		}
		matches := true
		for key, expected := range required {
			if len(expected) == 0 || query.Get(key) != expected[len(expected)-1] {
				matches = false
				break
			}
		}
		if matches {
			return operation, true
		}
	}
	return fallback, foundFallback
}
