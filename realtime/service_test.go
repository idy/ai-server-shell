package realtime_test

import (
	"context"

	"github.com/idy/ai-server-shell/realtime"
)

type stubService struct{}

func (stubService) Open(context.Context, realtime.OpenRequest) (realtime.Session, error) {
	return stubSession{events: make(chan realtime.ServerEvent)}, nil
}

type stubSession struct {
	events chan realtime.ServerEvent
}

func (stubSession) Handle(context.Context, realtime.ClientEvent) error { return nil }
func (s stubSession) Events() <-chan realtime.ServerEvent              { return s.events }
func (stubSession) Close(context.Context) error                        { return nil }

func ExampleService() {
	var service realtime.Service = stubService{}
	_, _ = service.Open(context.Background(), realtime.OpenRequest{
		SessionID: "session_example",
		Model:     "tutor",
	})

	// Output:
}
