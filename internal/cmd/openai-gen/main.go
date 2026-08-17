package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type document struct {
	Paths map[string]map[string]operation `json:"paths"`
}

type operation struct {
	OperationID string              `json:"operationId"`
	Tags        []string            `json:"tags"`
	RequestBody *requestBody        `json:"requestBody"`
	Responses   map[string]response `json:"responses"`
}

type requestBody struct {
	Content map[string]json.RawMessage `json:"content"`
}

type response struct {
	Content map[string]json.RawMessage `json:"content"`
}

type generatedOperation struct {
	Method        string
	Path          string
	OperationID   string
	Capability    string
	RequestMedia  []string
	ResponseMedia []string
	SuccessStatus int
	SDKCall       string
	LocalCase     string
	Transports    []string
	SchemaCases   []string
	LiveProfile   string
	LiveReason    string
}

type eventInventory struct {
	SDK      string                  `json:"sdk"`
	Surfaces map[string]eventSurface `json:"surfaces"`
}

type eventSurface struct {
	Client []string `json:"client"`
	Server []string `json:"server"`
}

type generatedEvent struct {
	Surface   string
	Direction string
	Type      string
}

func main() {
	specPath := flag.String("spec", "", "path to the pinned OpenAPI JSON")
	outputPath := flag.String("output", "", "generated Go output")
	manifestPath := flag.String("manifest", "", "generated SDK operation manifest")
	eventsPath := flag.String("events", "", "pinned WebSocket event inventory")
	compatibilityPath := flag.String("compatibility", "", "generated compatibility matrix")
	flag.Parse()
	if *specPath == "" || *outputPath == "" {
		fatalf("both -spec and -output are required")
	}

	raw, err := os.ReadFile(*specPath)
	if err != nil {
		fatalf("read spec: %v", err)
	}
	var spec document
	if err := json.Unmarshal(raw, &spec); err != nil {
		fatalf("decode spec: %v", err)
	}
	operations := collectOperations(spec)
	if len(spec.Paths) != 182 || len(operations) != 288 {
		fatalf("unexpected frozen profile size: paths=%d operations=%d", len(spec.Paths), len(operations))
	}

	var events eventInventory
	if *eventsPath != "" {
		eventData, err := os.ReadFile(*eventsPath)
		if err != nil {
			fatalf("read events: %v", err)
		}
		if err := json.Unmarshal(eventData, &events); err != nil {
			fatalf("decode events: %v", err)
		}
		if count := len(collectEvents(events)); count != 121 {
			fatalf("event inventory contains %d direction-specific entries, want 121", count)
		}
	}
	if err := validateEvidence(operations, events); err != nil {
		fatalf("validate evidence: %v", err)
	}
	generated, err := render(raw, len(spec.Paths), operations, events)
	if err != nil {
		fatalf("render: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(*outputPath), 0o755); err != nil {
		fatalf("create output directory: %v", err)
	}
	if err := os.WriteFile(*outputPath, generated, 0o644); err != nil {
		fatalf("write output: %v", err)
	}
	if *manifestPath != "" {
		manifest, err := json.MarshalIndent(operations, "", "  ")
		if err != nil {
			fatalf("encode manifest: %v", err)
		}
		manifest = append(manifest, '\n')
		if err := os.MkdirAll(filepath.Dir(*manifestPath), 0o755); err != nil {
			fatalf("create manifest directory: %v", err)
		}
		if err := os.WriteFile(*manifestPath, manifest, 0o644); err != nil {
			fatalf("write manifest: %v", err)
		}
	}
	if *compatibilityPath != "" {
		if *eventsPath == "" {
			fatalf("-events is required with -compatibility")
		}
		matrix := renderCompatibility(operations, events)
		if err := os.MkdirAll(filepath.Dir(*compatibilityPath), 0o755); err != nil {
			fatalf("create compatibility directory: %v", err)
		}
		if err := os.WriteFile(*compatibilityPath, matrix, 0o644); err != nil {
			fatalf("write compatibility matrix: %v", err)
		}
	}
}

func validateEvidence(operations []generatedOperation, inventory eventInventory) error {
	operationIDs := make(map[string]bool, len(operations))
	caseNames := make(map[string]bool, len(operations))
	rawSeen := make(map[string]bool, len(rawOnlyOperations))
	validProfiles := map[string]bool{"safe": true, "paid": true, "mutation": true, "unavailable": true}
	for _, operation := range operations {
		if operationIDs[operation.OperationID] {
			return fmt.Errorf("duplicate operation ID %q", operation.OperationID)
		}
		operationIDs[operation.OperationID] = true
		if operation.LocalCase == "" || caseNames[operation.LocalCase] {
			return fmt.Errorf("missing or duplicate local case %q", operation.LocalCase)
		}
		caseNames[operation.LocalCase] = true
		if operation.SDKCall != "resource_helper" && operation.SDKCall != "raw_sdk_exception" {
			return fmt.Errorf("operation %q has invalid SDK call %q", operation.OperationID, operation.SDKCall)
		}
		if operation.SDKCall == "raw_sdk_exception" {
			rawSeen[operation.OperationID] = true
		}
		if len(operation.Transports) == 0 || len(operation.SchemaCases) == 0 || !validProfiles[operation.LiveProfile] {
			return fmt.Errorf("operation %q has incomplete evidence metadata", operation.OperationID)
		}
	}
	for operationID := range rawOnlyOperations {
		if !rawSeen[operationID] {
			return fmt.Errorf("raw SDK exception %q is absent from the profile", operationID)
		}
	}
	eventKeys := make(map[string]bool)
	for _, event := range collectEvents(inventory) {
		key := event.Surface + "/" + event.Direction + "/" + event.Type
		if eventKeys[key] {
			return fmt.Errorf("duplicate event evidence %q", key)
		}
		eventKeys[key] = true
	}
	return nil
}

func collectOperations(spec document) []generatedOperation {
	methods := map[string]bool{"get": true, "post": true, "put": true, "patch": true, "delete": true}
	var result []generatedOperation
	for path, pathItem := range spec.Paths {
		for method, operation := range pathItem {
			if !methods[strings.ToLower(method)] {
				continue
			}
			if operation.OperationID == "" {
				fatalf("%s %s has no operationId", method, path)
			}
			item := generatedOperation{
				Method:      strings.ToUpper(method),
				Path:        path,
				OperationID: operation.OperationID,
				Capability:  capabilityFor(path, operation.Tags),
			}
			item.SuccessStatus = successStatus(operation.Responses)
			if operation.RequestBody != nil {
				for media := range operation.RequestBody.Content {
					item.RequestMedia = append(item.RequestMedia, media)
				}
			}
			for _, response := range operation.Responses {
				for media := range response.Content {
					item.ResponseMedia = appendUnique(item.ResponseMedia, media)
				}
			}
			sort.Strings(item.RequestMedia)
			sort.Strings(item.ResponseMedia)
			item.SDKCall = "resource_helper"
			if rawOnlyOperations[item.OperationID] {
				item.SDKCall = "raw_sdk_exception"
			}
			item.LocalCase = "http." + item.Capability + "." + item.OperationID
			item.Transports = operationTransports(item)
			item.SchemaCases = []string{"minimal_valid_request", "valid_success_response"}
			item.LiveProfile, item.LiveReason = liveClassification(item)
			result = append(result, item)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Path == result[j].Path {
			return result[i].Method < result[j].Method
		}
		return result[i].Path < result[j].Path
	})
	return result
}

var rawOnlyOperations = map[string]bool{
	"listVoiceConsents": true, "createVoiceConsent": true, "deleteVoiceConsent": true,
	"getVoiceConsent": true, "updateVoiceConsent": true, "createVoice": true,
	"create-realtime-call": true, "create-realtime-translation-client-secret": true,
}

func operationTransports(operation generatedOperation) []string {
	seen := map[string]bool{}
	add := func(value string) {
		if !seen[value] {
			operation.Transports = append(operation.Transports, value)
			seen[value] = true
		}
	}
	if strings.Contains(operation.Path, "{") {
		add("path")
	}
	if strings.Contains(operation.Path, "?") {
		add("query")
	}
	for _, media := range append(append([]string(nil), operation.RequestMedia...), operation.ResponseMedia...) {
		switch {
		case media == "multipart/form-data":
			add("multipart")
		case media == "text/event-stream":
			add("sse")
		case strings.Contains(media, "json"):
			add("json")
		default:
			add("binary")
		}
	}
	return operation.Transports
}

func liveClassification(operation generatedOperation) (string, string) {
	switch operation.OperationID {
	case "listModels":
		return "safe", "scheduled read-only differential"
	case "createEmbedding":
		return "paid", "manual cost approval required; no live result recorded"
	case "createFile", "deleteFile":
		return "mutation", "manual disposable-account approval and verified cleanup required; no live result recorded"
	default:
		return "unavailable", "no named bounded live differential is configured"
	}
}

func successStatus(responses map[string]response) int {
	result := 0
	for code := range responses {
		status, err := strconv.Atoi(code)
		if err == nil && status >= 200 && status < 300 && (result == 0 || status < result) {
			result = status
		}
	}
	if result == 0 {
		return 200
	}
	return result
}

func appendUnique(values []string, candidate string) []string {
	for _, value := range values {
		if value == candidate {
			return values
		}
	}
	return append(values, candidate)
}

func capabilityFor(path string, tags []string) string {
	tag := ""
	if len(tags) > 0 {
		tag = tags[0]
	}
	switch tag {
	case "Responses":
		return "responses"
	case "Chat":
		return "chat"
	case "Completions":
		return "completions"
	case "Conversations":
		return "conversations"
	case "Embeddings":
		return "embeddings"
	case "Audio":
		return "audio"
	case "Images":
		return "images"
	case "Videos":
		return "videos"
	case "Moderations":
		return "moderations"
	case "Models":
		return "models"
	case "Files":
		return "files"
	case "Uploads":
		return "uploads"
	case "Batch":
		return "batches"
	case "Assistants":
		return "assistants"
	case "Vector stores":
		return "vector_stores"
	case "Fine-tuning":
		return "fine_tuning"
	case "Evals":
		return "evals"
	case "Realtime":
		return "realtime"
	case "Skills":
		return "skills"
	}
	if strings.HasPrefix(path, "/containers") {
		return "containers"
	}
	if strings.HasPrefix(path, "/responses") {
		return "responses"
	}
	if strings.HasPrefix(path, "/chatkit") {
		return "chatkit"
	}
	if strings.HasPrefix(path, "/content_provenance_checks") {
		return "moderations"
	}
	if strings.HasPrefix(path, "/organization") || strings.HasPrefix(path, "/projects") {
		return "organization"
	}
	fatalf("cannot map capability for %s (tag %q)", path, tag)
	return ""
}

func render(raw []byte, pathCount int, operations []generatedOperation, inventory eventInventory) ([]byte, error) {
	var compressed bytes.Buffer
	zw, err := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(raw); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	digest := sha256.Sum256(raw)

	var out bytes.Buffer
	fmt.Fprintln(&out, "// Code generated by internal/cmd/openai-gen; DO NOT EDIT.")
	fmt.Fprintln(&out, "package profile")
	fmt.Fprintln(&out)
	fmt.Fprintf(&out, "const SpecPathCount = %d\n", pathCount)
	fmt.Fprintf(&out, "const SpecOperationCount = %d\n", len(operations))
	fmt.Fprintf(&out, "const SpecSHA256 = %q\n", hex.EncodeToString(digest[:]))
	fmt.Fprintf(&out, "const compressedSpecBase64 = %q\n\n", base64.StdEncoding.EncodeToString(compressed.Bytes()))
	fmt.Fprintln(&out, "var Operations = []Operation{")
	for _, operation := range operations {
		fmt.Fprintf(&out, "{Method:%q, Path:%q, ID:%q, Capability:%q, RequestMedia:%#v, ResponseMedia:%#v, SuccessStatus:%d},\n",
			operation.Method, operation.Path, operation.OperationID, operation.Capability,
			operation.RequestMedia, operation.ResponseMedia, operation.SuccessStatus)
	}
	fmt.Fprintln(&out, "}")
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "var Events = []Event{")
	for _, event := range collectEvents(inventory) {
		fmt.Fprintf(&out, "{Surface:%q, Direction:%q, Type:%q},\n", event.Surface, event.Direction, event.Type)
	}
	fmt.Fprintln(&out, "}")
	return format.Source(out.Bytes())
}

func collectEvents(inventory eventInventory) []generatedEvent {
	var result []generatedEvent
	for surface, events := range inventory.Surfaces {
		for _, eventType := range events.Client {
			result = append(result, generatedEvent{Surface: surface, Direction: "client", Type: eventType})
		}
		for _, eventType := range events.Server {
			result = append(result, generatedEvent{Surface: surface, Direction: "server", Type: eventType})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Surface != result[j].Surface {
			return result[i].Surface < result[j].Surface
		}
		if result[i].Direction != result[j].Direction {
			return result[i].Direction < result[j].Direction
		}
		return result[i].Type < result[j].Type
	})
	return result
}

func renderCompatibility(operations []generatedOperation, events eventInventory) []byte {
	var out bytes.Buffer
	fmt.Fprintln(&out, "# Code generated by internal/cmd/openai-gen; DO NOT EDIT.")
	fmt.Fprintln(&out, "profile: openai-m1")
	fmt.Fprintln(&out, "sdk:", strconv.Quote(events.SDK))
	fmt.Fprintln(&out, "http:")
	for _, operation := range operations {
		fmt.Fprintln(&out, "  - operation:", strconv.Quote(operation.OperationID))
		fmt.Fprintln(&out, "    method:", strconv.Quote(operation.Method))
		fmt.Fprintln(&out, "    path:", strconv.Quote(operation.Path))
		fmt.Fprintln(&out, "    capability:", strconv.Quote(operation.Capability))
		fmt.Fprintln(&out, "    route: covered")
		fmt.Fprintln(&out, "    local_semantic: validated_official_sdk")
		fmt.Fprintln(&out, "    local_case:", strconv.Quote(operation.LocalCase))
		fmt.Fprintln(&out, "    sdk_call:", operation.SDKCall)
		fmt.Fprintln(&out, "    transports:", yamlList(operation.Transports))
		fmt.Fprintln(&out, "    schema_cases:", yamlList(operation.SchemaCases))
		fmt.Fprintln(&out, "    live_profile:", operation.LiveProfile)
		fmt.Fprintln(&out, "    live_reason:", strconv.Quote(operation.LiveReason))
	}
	fmt.Fprintln(&out, "events:")
	for _, event := range collectEvents(events) {
		fmt.Fprintln(&out, "  - surface:", strconv.Quote(event.Surface))
		fmt.Fprintln(&out, "    direction:", strconv.Quote(event.Direction))
		fmt.Fprintln(&out, "    type:", strconv.Quote(event.Type))
		fmt.Fprintln(&out, "    capability: realtime")
		fmt.Fprintln(&out, "    route: covered")
		fmt.Fprintln(&out, "    local_semantic: official_sdk_"+map[string]string{"client": "send", "server": "listener"}[event.Direction])
		fmt.Fprintln(&out, "    local_case:", strconv.Quote("websocket."+event.Surface+"."+event.Direction+"."+event.Type))
		fmt.Fprintln(&out, "    transports: [websocket]")
		fmt.Fprintln(&out, "    schema_cases: [representative_payload, unknown_field_preserved]")
		fmt.Fprintln(&out, "    live_profile: unavailable")
		fmt.Fprintln(&out, "    live_reason: \"no bounded live WebSocket differential is configured\"")
	}
	return out.Bytes()
}

func yamlList(values []string) string {
	quoted := make([]string, len(values))
	for index, value := range values {
		quoted[index] = strconv.Quote(value)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
