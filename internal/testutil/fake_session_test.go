package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/idy/ai-server-shell/backend"
)

func TestFakeSessionCloseUnblocksSlowConsumer(t *testing.T) {
	session, err := (&FakeSessionBackend{}).OpenSession(context.Background(), backend.SessionRequest{Surface: backend.SessionRealtime})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 32 {
			_ = session.Handle(context.Background(), backend.Event{Type: "session.update"})
		}
	}()
	time.Sleep(10 * time.Millisecond)
	if err := session.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("session close did not unblock a producer behind a slow consumer")
	}
	select {
	case _, open := <-session.Events():
		for open {
			_, open = <-session.Events()
		}
	case <-time.After(time.Second):
		t.Fatal("session event stream did not close")
	}
}
