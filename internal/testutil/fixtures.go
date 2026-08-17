package testutil

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/idy/ai-server-shell/backend"
)

const (
	caseHeader              = "X-Ai-Shell-Case"
	responseStatusHeader    = "X-Ai-Shell-Response-Status"
	responseMediaTypeHeader = "X-Ai-Shell-Response-Media-Type"
	responseBodyHeader      = "X-Ai-Shell-Response-Body"
)

// ResponseFixture decodes the response selected by an official-SDK semantic
// case. The fixture headers are accepted only by test backends.
func ResponseFixture(request backend.Request) (backend.Response, bool, error) {
	caseName := firstHeader(request, caseHeader)
	if caseName == "" {
		return backend.Response{}, false, nil
	}
	if caseName != request.Operation {
		return backend.Response{}, true, fmt.Errorf("SDK case %q reached operation %q", caseName, request.Operation)
	}
	status, err := strconv.Atoi(firstHeader(request, responseStatusHeader))
	if err != nil || status < 200 || status > 299 {
		return backend.Response{}, true, fmt.Errorf("SDK case %q has invalid response status", caseName)
	}
	mediaType := firstHeader(request, responseMediaTypeHeader)
	body, err := base64.StdEncoding.DecodeString(firstHeader(request, responseBodyHeader))
	if err != nil {
		return backend.Response{}, true, fmt.Errorf("SDK case %q has invalid response body: %w", caseName, err)
	}
	response := backend.Response{StatusCode: status, MediaType: mediaType}
	if len(body) == 0 {
		return response, true, nil
	}
	if strings.Contains(mediaType, "json") {
		if !json.Valid(body) {
			return backend.Response{}, true, fmt.Errorf("SDK case %q response is not valid JSON", caseName)
		}
		response.JSON = body
	} else {
		response.Body = io.NopCloser(bytes.NewReader(body))
	}
	return response, true, nil
}

func firstHeader(request backend.Request, name string) string {
	values := request.Metadata.Extensions[name]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
