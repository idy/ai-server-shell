package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/idy/ai-server-shell/backend"
	"github.com/idy/ai-server-shell/openai/internal/contract"
)

func TestFrozenEventInventoryRoundTrips(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "spec", "openai", "realtime-events.json"))
	if err != nil {
		t.Fatal(err)
	}
	var inventory struct {
		SDK      string `json:"sdk"`
		Surfaces map[string]struct {
			Client []string `json:"client"`
			Server []string `json:"server"`
		} `json:"surfaces"`
	}
	if err := json.Unmarshal(data, &inventory); err != nil {
		t.Fatal(err)
	}
	if inventory.SDK != "openai-node@7.4.0" || len(inventory.Surfaces) != 3 {
		t.Fatalf("unexpected event profile: SDK=%q surfaces=%d", inventory.SDK, len(inventory.Surfaces))
	}
	seen := map[string]bool{}
	for surface, events := range inventory.Surfaces {
		for direction, values := range map[string][]string{"client": events.Client, "server": events.Server} {
			for _, eventType := range values {
				key := surface + "/" + direction + "/" + eventType
				if seen[key] {
					t.Fatalf("duplicate event %s", key)
				}
				seen[key] = true
				payload := []byte(`{"type":` + mustJSON(t, eventType) + `,"future_field":{"kept":true}}`)
				decoded, err := decodeEvent(payload)
				if err != nil {
					t.Fatalf("decode %s: %v", key, err)
				}
				encoded, err := encodeEvent(decoded)
				if err != nil {
					t.Fatalf("encode %s: %v", key, err)
				}
				var roundTrip map[string]any
				if err := json.Unmarshal(encoded, &roundTrip); err != nil || roundTrip["type"] != eventType || roundTrip["future_field"] == nil {
					t.Fatalf("round trip %s did not preserve its discriminator and unknown field: %s", key, encoded)
				}
			}
		}
	}
	if got, want := len(seen), 121; got != want {
		t.Fatalf("covered event entries = %d, want %d", got, want)
	}
}

func mustJSON(t *testing.T, value string) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestDecodeEventRejectsMalformedFrames(t *testing.T) {
	for _, input := range [][]byte{[]byte(`not-json`), []byte(`{}`), []byte(`{"type":""}`)} {
		if _, err := decodeEvent(input); err == nil {
			t.Fatalf("decodeEvent(%q) succeeded", input)
		}
	}
}

func TestHandlerWebSocketLifecycle(t *testing.T) {
	sessions := &echoSessionBackend{}
	services, _ := backend.NewServices(backend.WithRealtime(sessions))
	server := httptest.NewServer(New(services, contract.Config{
		BasePath: "/v1", MaxBodyBytes: 1024,
		Authenticate: func(context.Context, *http.Request, string) (string, error) { return "caller", nil },
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	connection, response, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/v1/realtime?model=test", &websocket.DialOptions{Subprotocols: []string{"realtime", "openai-insecure-api-key.test"}})
	if err != nil {
		t.Fatalf("dial: %v; response=%v", err, response)
	}
	if connection.Subprotocol() != "realtime" {
		t.Fatalf("subprotocol = %q", connection.Subprotocol())
	}
	if err := connection.Write(ctx, websocket.MessageText, []byte(`{"type":"session.update","future":true}`)); err != nil {
		t.Fatal(err)
	}
	_, data, err := connection.Read(ctx)
	if err != nil || !strings.Contains(string(data), `"type":"session.updated"`) {
		t.Fatalf("read = %s, %v", data, err)
	}
	_ = connection.Close(websocket.StatusNormalClosure, "done")
	select {
	case <-sessions.closed:
	case <-ctx.Done():
		t.Fatal("session was not closed")
	}
	if sessions.request.Parameters["model"] != "test" || sessions.request.Metadata.CallerID != "caller" {
		t.Fatalf("session request = %#v", sessions.request)
	}
}

func TestHandlerRejectsBeforeUpgrade(t *testing.T) {
	services, _ := backend.NewServices()
	tests := []struct {
		name   string
		auth   contract.Authenticate
		status int
	}{
		{"missing session", func(context.Context, *http.Request, string) (string, error) { return "caller", nil }, http.StatusNotImplemented},
		{"auth", func(context.Context, *http.Request, string) (string, error) { return "", errors.New("denied") }, http.StatusUnauthorized},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(New(services, contract.Config{BasePath: "/v1", MaxBodyBytes: 1024, Authenticate: test.auth}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			connection, response, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/v1/realtime", nil)
			if connection != nil {
				connection.CloseNow()
			}
			if err == nil || response == nil || response.StatusCode != test.status {
				t.Fatalf("dial err=%v response=%v", err, response)
			}
			response.Body.Close()
		})
	}
}

func TestHandlerRejectsNilSession(t *testing.T) {
	services, _ := backend.NewServices(backend.WithRealtime(nilSessionBackend{}))
	server := httptest.NewServer(New(services, contract.Config{BasePath: "/v1", MaxBodyBytes: 1024, Authenticate: func(context.Context, *http.Request, string) (string, error) { return "caller", nil }}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	connection, response, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/v1/realtime", nil)
	if connection != nil {
		connection.CloseNow()
	}
	if err == nil || response == nil || response.StatusCode != http.StatusInternalServerError {
		t.Fatalf("dial err=%v response=%v", err, response)
	}
	response.Body.Close()
}

type nilSessionBackend struct{}

func (nilSessionBackend) OpenSession(context.Context, backend.SessionRequest) (backend.Session, error) {
	return nil, nil
}

func TestHandlerClosesMalformedFrame(t *testing.T) {
	services, _ := backend.NewServices(backend.WithRealtime(&echoSessionBackend{}))
	server := httptest.NewServer(New(services, contract.Config{BasePath: "/v1", MaxBodyBytes: 1024, Authenticate: func(context.Context, *http.Request, string) (string, error) { return "caller", nil }}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	connection, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/v1/realtime", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := connection.Write(ctx, websocket.MessageBinary, []byte("bad")); err != nil {
		t.Fatal(err)
	}
	_, _, err = connection.Read(ctx)
	if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("close = %v, want policy violation", err)
	}
}

type echoSessionBackend struct {
	mu      sync.Mutex
	request backend.SessionRequest
	closed  chan struct{}
}

func (b *echoSessionBackend) OpenSession(_ context.Context, request backend.SessionRequest) (backend.Session, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.request = request
	b.closed = make(chan struct{})
	return &echoSession{events: make(chan backend.Event, 1), closed: b.closed}, nil
}

type echoSession struct {
	events chan backend.Event
	closed chan struct{}
	once   sync.Once
}

func (s *echoSession) Handle(_ context.Context, event backend.Event) error {
	s.events <- backend.Event{Type: "session.updated", Data: json.RawMessage(`{"type":"session.updated"}`)}
	return nil
}
func (s *echoSession) Events() <-chan backend.Event { return s.events }
func (s *echoSession) Close(context.Context) error {
	s.once.Do(func() { close(s.events); close(s.closed) })
	return nil
}

func TestCodecAndCloseHelpers(t *testing.T) {
	if _, err := encodeEvent(backend.Event{}); err == nil {
		t.Fatal("empty event type was accepted")
	}
	if _, err := encodeEvent(backend.Event{Type: "one", Data: []byte(`{"type":"two"}`)}); err == nil {
		t.Fatal("mismatched event type was accepted")
	}
	if _, err := encodeEvent(backend.Event{Type: "one", Data: []byte(`bad`)}); err == nil {
		t.Fatal("invalid event JSON was accepted")
	}
	if code, _ := closeReason(protocolError("bad")); code != websocket.StatusPolicyViolation {
		t.Fatalf("protocol close = %d", code)
	}
	if code, _ := closeReason(errors.New("backend")); code != websocket.StatusInternalError {
		t.Fatalf("backend close = %d", code)
	}
	if len(truncateReason(strings.Repeat("x", 200))) != 120 {
		t.Fatal("close reason was not truncated")
	}
}

func TestFrozenEventAllowlist(t *testing.T) {
	for _, test := range []struct {
		surface   backend.SessionSurface
		direction string
		eventType string
		want      bool
	}{
		{backend.SessionRealtime, "client", "session.update", true},
		{backend.SessionRealtime, "client", "session.close", true},
		{backend.SessionRealtime, "server", "response.done", true},
		{backend.SessionResponsesSocket, "client", "response.create", true},
		{backend.SessionResponsesSocket, "server", "response.completed", true},
		{backend.SessionResponsesSocket, "client", "future.event", false},
	} {
		if got := eventAllowed(test.surface, test.direction, test.eventType); got != test.want {
			t.Fatalf("eventAllowed(%s, %s, %s) = %v", test.surface, test.direction, test.eventType, got)
		}
	}
}

func TestShutdownCancelsActiveSessionAndRejectsNewConnections(t *testing.T) {
	sessions := &echoSessionBackend{}
	services, _ := backend.NewServices(backend.WithRealtime(sessions))
	handler := New(services, contract.Config{BasePath: "/v1", MaxBodyBytes: 1024, Authenticate: func(context.Context, *http.Request, string) (string, error) { return "caller", nil }})
	server := httptest.NewServer(handler)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	connection, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/v1/realtime", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := handler.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	_, _, err = connection.Read(ctx)
	if err == nil {
		t.Fatal("connection remained open after shutdown")
	}
	newConnection, response, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/v1/realtime", nil)
	if newConnection != nil {
		newConnection.CloseNow()
	}
	if err == nil || response == nil || response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("post-shutdown dial err=%v response=%v", err, response)
	}
	response.Body.Close()
}
