package aiservershell

import (
	"fmt"
	"net/http"
	"reflect"
)

// Server is an optional aggregate for independently constructed protocol
// handlers. It implements http.Handler and does not own network listeners.
type Server struct {
	mux *http.ServeMux
}

// ServerOption configures an aggregate Server.
type ServerOption func(*serverConfig) error

type serverConfig struct {
	handlers []mountedHandler
}

type mountedHandler struct {
	pattern string
	handler http.Handler
}

// New constructs an aggregate Server.
func New(options ...ServerOption) (*Server, error) {
	config := serverConfig{}
	for _, option := range options {
		if option == nil {
			return nil, fmt.Errorf("aiservershell: nil option")
		}
		if err := option(&config); err != nil {
			return nil, err
		}
	}
	mux := http.NewServeMux()
	for _, mounted := range config.handlers {
		mux.Handle(mounted.pattern, mounted.handler)
	}
	return &Server{mux: mux}, nil
}

// WithHandler mounts one independently constructed protocol handler. Pattern
// follows http.ServeMux syntax.
func WithHandler(pattern string, handler http.Handler) ServerOption {
	return func(config *serverConfig) error {
		if pattern == "" {
			return fmt.Errorf("aiservershell: empty handler pattern")
		}
		if isNilHandler(handler) {
			return fmt.Errorf("aiservershell: nil handler for %q", pattern)
		}
		if err := validatePattern(pattern); err != nil {
			return err
		}
		for _, existing := range config.handlers {
			if existing.pattern == pattern {
				return fmt.Errorf("aiservershell: handler pattern %q already registered", pattern)
			}
		}
		config.handlers = append(config.handlers, mountedHandler{pattern: pattern, handler: handler})
		return nil
	}
}

func isNilHandler(handler http.Handler) bool {
	if handler == nil {
		return true
	}
	value := reflect.ValueOf(handler)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func validatePattern(pattern string) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("aiservershell: invalid handler pattern %q: %v", pattern, recovered)
		}
	}()
	http.NewServeMux().Handle(pattern, http.NotFoundHandler())
	return nil
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	s.mux.ServeHTTP(writer, request)
}
