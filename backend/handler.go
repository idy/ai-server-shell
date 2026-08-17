package backend

import "context"

// Handler executes protocol-neutral backend operations. A single concrete
// Handler can be registered for all capabilities, or individual capabilities
// can override it.
type Handler interface {
	Handle(context.Context, Request) (Response, error)
}

// HandlerFunc adapts a function to Handler.
type HandlerFunc func(context.Context, Request) (Response, error)

func (f HandlerFunc) Handle(ctx context.Context, req Request) (Response, error) {
	return f(ctx, req)
}
