package backend

import (
	"fmt"
	"reflect"
)

// Services is an immutable backend registry. Construct it with NewServices.
type Services struct {
	defaultHandler Handler
	handlers       map[Capability]Handler
	sessions       map[SessionSurface]SessionBackend
}

// Option configures a Services registry.
type Option func(*serviceConfig) error

type serviceConfig struct {
	defaultHandler Handler
	handlers       map[Capability]Handler
	sessions       map[SessionSurface]SessionBackend
}

// NewServices validates and freezes backend injection.
func NewServices(options ...Option) (Services, error) {
	cfg := serviceConfig{
		handlers: make(map[Capability]Handler),
		sessions: make(map[SessionSurface]SessionBackend),
	}
	for _, option := range options {
		if option == nil {
			return Services{}, fmt.Errorf("backend: nil option")
		}
		if err := option(&cfg); err != nil {
			return Services{}, err
		}
	}

	handlers := make(map[Capability]Handler, len(cfg.handlers))
	for capability, handler := range cfg.handlers {
		handlers[capability] = handler
	}
	sessions := make(map[SessionSurface]SessionBackend, len(cfg.sessions))
	for surface, session := range cfg.sessions {
		sessions[surface] = session
	}
	return Services{defaultHandler: cfg.defaultHandler, handlers: handlers, sessions: sessions}, nil
}

// WithHandler registers one default backend for every capability that has no
// explicit override.
func WithHandler(handler Handler) Option {
	return func(cfg *serviceConfig) error {
		if isNilInterface(handler) {
			return fmt.Errorf("backend: nil default handler")
		}
		if cfg.defaultHandler != nil {
			return fmt.Errorf("backend: default handler already registered")
		}
		cfg.defaultHandler = handler
		return nil
	}
}

// WithCapability registers a backend override for one capability.
func WithCapability(capability Capability, handler Handler) Option {
	return func(cfg *serviceConfig) error {
		if !validCapability(capability) {
			return fmt.Errorf("backend: unknown capability %q", capability)
		}
		if isNilInterface(handler) {
			return fmt.Errorf("backend: nil handler for %s", capability)
		}
		if _, exists := cfg.handlers[capability]; exists {
			return fmt.Errorf("backend: capability %s already registered", capability)
		}
		cfg.handlers[capability] = handler
		return nil
	}
}

// Capability-specific helpers keep application wiring explicit while sharing
// the same protocol-neutral Handler contract. One object may be passed to any
// or all of these options.
func WithResponses(h Handler) Option     { return WithCapability(CapabilityResponses, h) }
func WithChat(h Handler) Option          { return WithCapability(CapabilityChat, h) }
func WithCompletions(h Handler) Option   { return WithCapability(CapabilityCompletions, h) }
func WithEmbeddings(h Handler) Option    { return WithCapability(CapabilityEmbeddings, h) }
func WithAudio(h Handler) Option         { return WithCapability(CapabilityAudio, h) }
func WithImages(h Handler) Option        { return WithCapability(CapabilityImages, h) }
func WithVideos(h Handler) Option        { return WithCapability(CapabilityVideos, h) }
func WithModerations(h Handler) Option   { return WithCapability(CapabilityModerations, h) }
func WithModels(h Handler) Option        { return WithCapability(CapabilityModels, h) }
func WithFiles(h Handler) Option         { return WithCapability(CapabilityFiles, h) }
func WithUploads(h Handler) Option       { return WithCapability(CapabilityUploads, h) }
func WithBatches(h Handler) Option       { return WithCapability(CapabilityBatches, h) }
func WithAssistants(h Handler) Option    { return WithCapability(CapabilityAssistants, h) }
func WithVectorStores(h Handler) Option  { return WithCapability(CapabilityVectorStores, h) }
func WithFineTuning(h Handler) Option    { return WithCapability(CapabilityFineTuning, h) }
func WithEvals(h Handler) Option         { return WithCapability(CapabilityEvals, h) }
func WithContainers(h Handler) Option    { return WithCapability(CapabilityContainers, h) }
func WithSkills(h Handler) Option        { return WithCapability(CapabilitySkills, h) }
func WithChatKit(h Handler) Option       { return WithCapability(CapabilityChatKit, h) }
func WithOrganization(h Handler) Option  { return WithCapability(CapabilityOrganization, h) }
func WithConversations(h Handler) Option { return WithCapability(CapabilityConversations, h) }

// WithRealtime registers the bidirectional Realtime session backend.
func WithRealtime(service SessionBackend) Option { return WithSession(SessionRealtime, service) }

// WithResponsesWebSocket registers the bidirectional Responses session backend.
func WithResponsesWebSocket(service SessionBackend) Option {
	return WithSession(SessionResponsesSocket, service)
}

// WithSession registers a backend for one bidirectional session surface.
func WithSession(surface SessionSurface, service SessionBackend) Option {
	return func(cfg *serviceConfig) error {
		if surface != SessionRealtime && surface != SessionResponsesSocket {
			return fmt.Errorf("backend: unknown session surface %q", surface)
		}
		if isNilInterface(service) {
			return fmt.Errorf("backend: nil session backend for %s", surface)
		}
		if _, exists := cfg.sessions[surface]; exists {
			return fmt.Errorf("backend: session surface %s already registered", surface)
		}
		cfg.sessions[surface] = service
		return nil
	}
}

// HandlerFor returns the explicit capability backend or the default backend.
func (s Services) HandlerFor(capability Capability) (Handler, bool) {
	handler, ok := s.handlers[capability]
	if ok {
		return handler, true
	}
	return s.defaultHandler, s.defaultHandler != nil
}

// SessionFor returns the registered session backend.
func (s Services) SessionFor(surface SessionSurface) (SessionBackend, bool) {
	service, ok := s.sessions[surface]
	return service, ok
}

func validCapability(candidate Capability) bool {
	for _, capability := range Capabilities {
		if candidate == capability {
			return true
		}
	}
	return false
}

func isNilInterface(value any) bool {
	if value == nil {
		return true
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}
