package openai

import (
	"fmt"
	"path"
	"reflect"
	"strings"
)

const defaultMaxBodyBytes int64 = 32 << 20

type settings struct {
	basePath       string
	maxBodyBytes   int64
	originPatterns []string
	authenticator  Authenticator
	validate       bool
}

// Option configures an OpenAI handler.
type Option func(*settings) error

// WithBasePath changes the default /v1 route prefix.
func WithBasePath(basePath string) Option {
	return func(settings *settings) error {
		if basePath == "" || !strings.HasPrefix(basePath, "/") {
			return fmt.Errorf("openai: base path must be absolute")
		}
		cleaned := path.Clean(basePath)
		if cleaned == "." || cleaned == "/" {
			return fmt.Errorf("openai: base path must not be root")
		}
		settings.basePath = strings.TrimSuffix(cleaned, "/")
		return nil
	}
}

// WithMaxBodyBytes sets the maximum buffered request body size.
func WithMaxBodyBytes(limit int64) Option {
	return func(settings *settings) error {
		if limit <= 0 {
			return fmt.Errorf("openai: body limit must be positive")
		}
		settings.maxBodyBytes = limit
		return nil
	}
}

// WithAuthenticator installs application-owned credential validation. Without
// this option the handler accepts requests and supplies an anonymous principal.
func WithAuthenticator(authenticator Authenticator) Option {
	return func(settings *settings) error {
		if isNil(authenticator) {
			return fmt.Errorf("openai: nil authenticator")
		}
		settings.authenticator = authenticator
		return nil
	}
}

func isNil(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

// WithOriginPatterns authorizes browser WebSocket origins.
func WithOriginPatterns(patterns ...string) Option {
	return func(settings *settings) error {
		for _, pattern := range patterns {
			if strings.TrimSpace(pattern) == "" {
				return fmt.Errorf("openai: empty origin pattern")
			}
		}
		settings.originPatterns = append([]string(nil), patterns...)
		return nil
	}
}

// WithoutSchemaValidation disables OpenAPI request/response validation. It is
// intended only for diagnostics and forward-compatibility experiments.
func WithoutSchemaValidation() Option {
	return func(settings *settings) error {
		settings.validate = false
		return nil
	}
}
