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

// RouterSpecJSON returns a semantically equivalent document accepted by the
// Go 1.24-compatible kin-openapi release. OpenAPI 3.1 permits type: "null";
// older kin-openapi represents the same constraint as nullable plus a null
// enum. The vendored source and its digest remain untouched.
func RouterSpecJSON() ([]byte, error) {
	raw, err := SpecJSON()
	if err != nil {
		return nil, err
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("decode frozen OpenAPI document: %w", err)
	}
	normalizeNullSchemas(value)
	result, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode router OpenAPI document: %w", err)
	}
	return result, nil
}

func normalizeNullSchemas(value any) {
	switch typed := value.(type) {
	case map[string]any:
		if schemaType, ok := typed["type"].(string); ok && schemaType == "null" {
			delete(typed, "type")
			typed["nullable"] = true
			if _, exists := typed["enum"]; !exists {
				typed["enum"] = []any{nil}
			}
		}
		for _, child := range typed {
			normalizeNullSchemas(child)
		}
	case []any:
		for _, child := range typed {
			normalizeNullSchemas(child)
		}
	}
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
