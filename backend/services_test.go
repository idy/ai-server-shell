package backend_test

import (
	"context"
	"testing"

	"github.com/idy/ai-server-shell/backend"
)

type stubHandler struct{}

func (*stubHandler) Handle(context.Context, backend.Request) (backend.Response, error) {
	return backend.Response{}, nil
}

type stubSessionBackend struct{}

func (*stubSessionBackend) OpenSession(context.Context, backend.SessionRequest) (backend.Session, error) {
	return nil, nil
}

func TestServicesDefaultAndOverride(t *testing.T) {
	defaultBackend := &stubHandler{}
	override := &stubHandler{}
	services, err := backend.NewServices(
		backend.WithHandler(defaultBackend),
		backend.WithCapability(backend.CapabilityResponses, override),
	)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := services.HandlerFor(backend.CapabilityResponses)
	if !ok || got != override {
		t.Fatalf("responses handler = %#v, %v", got, ok)
	}
	got, ok = services.HandlerFor(backend.CapabilityModels)
	if !ok || got != defaultBackend {
		t.Fatalf("models handler = %#v, %v", got, ok)
	}
}

func TestServicesRejectsTypedNilAndDuplicates(t *testing.T) {
	var nilHandler *stubHandler
	if _, err := backend.NewServices(backend.WithHandler(nilHandler)); err == nil {
		t.Fatal("typed nil handler was accepted")
	}
	handler := &stubHandler{}
	if _, err := backend.NewServices(
		backend.WithCapability(backend.CapabilityChat, handler),
		backend.WithCapability(backend.CapabilityChat, handler),
	); err == nil {
		t.Fatal("duplicate capability was accepted")
	}
}

func TestServicesSessionRegistration(t *testing.T) {
	session := &stubSessionBackend{}
	services, err := backend.NewServices(
		backend.WithRealtime(session),
		backend.WithResponsesWebSocket(session),
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, surface := range []backend.SessionSurface{backend.SessionRealtime, backend.SessionResponsesSocket} {
		got, ok := services.SessionFor(surface)
		if !ok || got != session {
			t.Fatalf("session %s = %#v, %v", surface, got, ok)
		}
	}
}

func TestServicesRejectsInvalidConfiguration(t *testing.T) {
	handler := &stubHandler{}
	session := &stubSessionBackend{}
	var nilSession *stubSessionBackend
	tests := []struct {
		name    string
		options []backend.Option
	}{
		{"nil option", []backend.Option{nil}},
		{"duplicate default", []backend.Option{backend.WithHandler(handler), backend.WithHandler(handler)}},
		{"unknown capability", []backend.Option{backend.WithCapability("future", handler)}},
		{"nil capability", []backend.Option{backend.WithModels((*stubHandler)(nil))}},
		{"unknown session", []backend.Option{backend.WithSession("future", session)}},
		{"nil session", []backend.Option{backend.WithRealtime(nilSession)}},
		{"duplicate session", []backend.Option{backend.WithRealtime(session), backend.WithRealtime(session)}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := backend.NewServices(test.options...); err == nil {
				t.Fatal("invalid configuration was accepted")
			}
		})
	}
}
