package openai

import "context"

// Credential is a redaction-safe authentication input. Token must never be
// logged or retained after Authenticate returns.
type Credential struct {
	Token     string
	Source    string
	RequestID string
}

// Principal identifies the authenticated caller delivered to backend metadata.
type Principal struct {
	ID string
}

// Authenticator validates an OpenAI-compatible credential.
type Authenticator interface {
	Authenticate(context.Context, Credential) (Principal, error)
}

// AuthenticatorFunc adapts a function to Authenticator.
type AuthenticatorFunc func(context.Context, Credential) (Principal, error)

func (f AuthenticatorFunc) Authenticate(ctx context.Context, credential Credential) (Principal, error) {
	return f(ctx, credential)
}
