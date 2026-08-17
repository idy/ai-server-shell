package testutil

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"reflect"
	"sort"
	"strings"

	"github.com/idy/ai-server-shell/backend"
)

const caseHeader = "X-Ai-Shell-Case"

// SemanticFixture is the backend-owned half of one official-SDK case. It is
// loaded before the test server starts, so an individual request cannot choose
// its expected request shape or response payload.
type SemanticFixture struct {
	Operation      string            `json:"operation"`
	Capability     string            `json:"capability"`
	RequestMedia   string            `json:"request_media"`
	Parameters     map[string]string `json:"parameters"`
	BodyJSON       json.RawMessage   `json:"body_json"`
	BodyFields     []string          `json:"body_fields"`
	ResponseStatus int               `json:"response_status"`
	ResponseMedia  string            `json:"response_media"`
	ResponseBody   string            `json:"response_body"`
}

func (f SemanticFixture) assertRequest(request backend.Request) error {
	if request.Operation != f.Operation || string(request.Capability) != f.Capability {
		return fmt.Errorf("case %q reached %s/%s", f.Operation, request.Capability, request.Operation)
	}
	if request.Input.MediaType != f.RequestMedia {
		return fmt.Errorf("case %q media type = %q, want %q", f.Operation, request.Input.MediaType, f.RequestMedia)
	}
	if err := assertParameters(f, request); err != nil {
		return err
	}
	if len(f.BodyJSON) > 0 {
		var want, got any
		if json.Unmarshal(f.BodyJSON, &want) != nil || json.Unmarshal(request.Input.Bytes, &got) != nil || !reflect.DeepEqual(want, got) {
			return fmt.Errorf("case %q JSON body does not match its backend fixture: got=%s want=%s", f.Operation, request.Input.Bytes, f.BodyJSON)
		}
	}
	if len(f.BodyFields) > 0 {
		if err := assertMultipartFields(f, request); err != nil {
			return err
		}
	}
	if f.RequestMedia != "" && !strings.Contains(f.RequestMedia, "json") && f.RequestMedia != "multipart/form-data" && len(request.Input.Bytes) == 0 {
		return fmt.Errorf("case %q has an empty binary body", f.Operation)
	}
	return nil
}

func assertParameters(f SemanticFixture, request backend.Request) error {
	if len(request.Parameters) != len(f.Parameters) {
		return fmt.Errorf("case %q parameters = %v, want %v", f.Operation, request.Parameters, f.Parameters)
	}
	for name, want := range f.Parameters {
		var got string
		if json.Unmarshal(request.Parameters[name], &got) != nil || got != want {
			return fmt.Errorf("case %q parameter %q = %q, want %q", f.Operation, name, got, want)
		}
	}
	return nil
}

func assertMultipartFields(f SemanticFixture, request backend.Request) error {
	contentType := firstHeader(request, "Content-Type")
	_, parameters, err := mime.ParseMediaType(contentType)
	if err != nil || parameters["boundary"] == "" {
		return fmt.Errorf("case %q has invalid multipart content type %q", f.Operation, contentType)
	}
	reader := multipart.NewReader(bytes.NewReader(request.Input.Bytes), parameters["boundary"])
	seen := make(map[string]bool)
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("case %q multipart body: %w", f.Operation, err)
		}
		name := strings.TrimSuffix(part.FormName(), "[]")
		name, _, _ = strings.Cut(name, "[")
		data, err := io.ReadAll(part)
		part.Close()
		if err != nil || len(data) == 0 {
			return fmt.Errorf("case %q multipart field %q is empty", f.Operation, name)
		}
		seen[name] = true
	}
	want := append([]string(nil), f.BodyFields...)
	sort.Strings(want)
	for _, name := range want {
		if !seen[name] {
			return fmt.Errorf("case %q is missing multipart field %q", f.Operation, name)
		}
	}
	return nil
}

func (f SemanticFixture) response() (backend.Response, error) {
	body, err := base64.StdEncoding.DecodeString(f.ResponseBody)
	if err != nil {
		return backend.Response{}, fmt.Errorf("case %q has invalid response fixture: %w", f.Operation, err)
	}
	response := backend.Response{StatusCode: f.ResponseStatus, MediaType: f.ResponseMedia}
	if len(body) == 0 {
		return response, nil
	}
	if strings.Contains(f.ResponseMedia, "json") {
		if !json.Valid(body) {
			return backend.Response{}, fmt.Errorf("case %q response is not valid JSON", f.Operation)
		}
		response.JSON = body
	} else {
		response.Body = io.NopCloser(bytes.NewReader(body))
	}
	return response, nil
}

func firstHeader(request backend.Request, name string) string {
	values := request.Metadata.Extensions[name]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
