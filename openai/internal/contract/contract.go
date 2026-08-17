package contract

import (
	"context"
	"net/http"
)

type Authenticate func(context.Context, *http.Request, string) (string, error)

type Config struct {
	BasePath       string
	MaxBodyBytes   int64
	OriginPatterns []string
	Validate       bool
	Authenticate   Authenticate
}
