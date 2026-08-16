package websocket

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/idy/ai-server-shell/backend"
	"github.com/idy/ai-server-shell/openai/internal/contract"
)

type handler struct {
	services backend.Services
	config   contract.Config
}

func New(services backend.Services, config contract.Config) http.Handler {
	return &handler{services: services, config: config}
}

func (h *handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	requestID := request.Header.Get("X-Request-Id")
	if requestID == "" {
		requestID = newRequestID()
	}
	callerID, err := h.config.Authenticate(request.Context(), request, requestID)
	if err != nil {
		writeHTTPError(writer, requestID, http.StatusUnauthorized, "authentication_error", "Invalid authentication credentials.")
		return
	}

	surface := backend.SessionRealtime
	if request.URL.Path == h.config.BasePath+"/responses" {
		surface = backend.SessionResponsesSocket
	}
	sessionBackend, ok := h.services.SessionFor(surface)
	if !ok {
		writeHTTPError(writer, requestID, http.StatusNotImplemented, "unsupported_error", "The requested WebSocket capability is not configured.")
		return
	}
	parameters := make(map[string]string, len(request.URL.Query()))
	for key, values := range request.URL.Query() {
		if len(values) > 0 {
			parameters[key] = values[len(values)-1]
		}
	}
	session, err := sessionBackend.OpenSession(request.Context(), backend.SessionRequest{
		Surface:    surface,
		Metadata:   backend.Metadata{RequestID: requestID, CallerID: callerID, Protocol: "openai"},
		Parameters: parameters,
	})
	if err != nil {
		if !isNilSession(session) {
			closeSession(session)
		}
		writeHTTPError(writer, requestID, http.StatusInternalServerError, "server_error", "Could not open the backend session.")
		return
	}
	if isNilSession(session) {
		writeHTTPError(writer, requestID, http.StatusInternalServerError, "server_error", "Could not open the backend session.")
		return
	}

	connection, err := websocket.Accept(writer, request, &websocket.AcceptOptions{
		Subprotocols: []string{"realtime"}, OriginPatterns: h.config.OriginPatterns,
	})
	if err != nil {
		closeSession(session)
		return
	}
	connection.SetReadLimit(h.config.MaxBodyBytes)

	ctx, cancel := context.WithCancel(request.Context())
	defer cancel()
	var closeOnce sync.Once
	closeAll := func(code websocket.StatusCode, reason string) {
		closeOnce.Do(func() {
			cancel()
			closeSession(session)
			_ = connection.Close(code, reason)
		})
	}
	defer closeAll(websocket.StatusNormalClosure, "OK")

	errorsChannel := make(chan error, 2)
	go func() { errorsChannel <- readLoop(ctx, connection, session) }()
	go func() { errorsChannel <- writeLoop(ctx, connection, session) }()
	err = <-errorsChannel
	code, reason := closeReason(err)
	closeAll(code, reason)
	select {
	case <-errorsChannel:
	case <-time.After(2 * time.Second):
	}
}

func isNilSession(session backend.Session) bool {
	if session == nil {
		return true
	}
	value := reflect.ValueOf(session)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func readLoop(ctx context.Context, connection *websocket.Conn, session backend.Session) error {
	for {
		messageType, data, err := connection.Read(ctx)
		if err != nil {
			return err
		}
		if messageType != websocket.MessageText {
			return protocolError("binary frames are not supported")
		}
		event, err := decodeEvent(data)
		if err != nil {
			return err
		}
		if err := session.Handle(ctx, event); err != nil {
			return err
		}
	}
}

func writeLoop(ctx context.Context, connection *websocket.Conn, session backend.Session) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, open := <-session.Events():
			if !open {
				return nil
			}
			data, err := encodeEvent(event)
			if err != nil {
				return err
			}
			if err := connection.Write(ctx, websocket.MessageText, data); err != nil {
				return err
			}
		}
	}
}

func decodeEvent(data []byte) (backend.Event, error) {
	var envelope struct {
		Type    string `json:"type"`
		EventID string `json:"event_id"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return backend.Event{}, protocolError("event is not valid JSON")
	}
	if strings.TrimSpace(envelope.Type) == "" {
		return backend.Event{}, protocolError("event type is required")
	}
	return backend.Event{Type: envelope.Type, ID: envelope.EventID, Data: append(json.RawMessage(nil), data...)}, nil
}

func encodeEvent(event backend.Event) ([]byte, error) {
	if strings.TrimSpace(event.Type) == "" {
		return nil, protocolError("backend event type is required")
	}
	if len(event.Data) == 0 {
		return json.Marshal(map[string]string{"type": event.Type, "event_id": event.ID})
	}
	var envelope map[string]any
	if err := json.Unmarshal(event.Data, &envelope); err != nil {
		return nil, protocolError("backend event is not valid JSON")
	}
	if existing, ok := envelope["type"].(string); ok && existing != event.Type {
		return nil, protocolError("backend event type does not match its payload")
	}
	envelope["type"] = event.Type
	if event.ID != "" {
		envelope["event_id"] = event.ID
	}
	return json.Marshal(envelope)
}

type protocolFailure struct{ message string }

func (e *protocolFailure) Error() string { return e.message }

func protocolError(message string) error { return &protocolFailure{message: message} }

func closeReason(err error) (websocket.StatusCode, string) {
	if err == nil || errors.Is(err, context.Canceled) {
		return websocket.StatusNormalClosure, "OK"
	}
	var protocol *protocolFailure
	if errors.As(err, &protocol) {
		return websocket.StatusPolicyViolation, truncateReason(protocol.message)
	}
	status := websocket.CloseStatus(err)
	if status != -1 {
		return status, "peer closed"
	}
	return websocket.StatusInternalError, "backend session failed"
}

func truncateReason(reason string) string {
	if len(reason) > 120 {
		return reason[:120]
	}
	return reason
}

func closeSession(session backend.Session) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = session.Close(ctx)
}

func writeHTTPError(writer http.ResponseWriter, requestID string, status int, errorType, message string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("X-Request-Id", requestID)
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]any{"error": map[string]any{
		"message": message, "type": errorType, "param": nil, "code": nil,
	}})
}

func newRequestID() string {
	var value [12]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "req_unknown"
	}
	return "req_" + hex.EncodeToString(value[:])
}
