package enginelink

import (
	"context"
	"errors"
	"fmt"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

var (
	ErrWorkspaceIDRequired = errors.New("workspace id is required for event stream")

	ErrNoConnection = errors.New("engine connection unavailable")

	ErrTransportNotWired = errors.New("event stream unavailable: transport not wired")
)

var StreamKinds = []string{
	"message", "run_complete", "permission_request", "question_batch_request", "file",
}

type EventConsumer interface {
	Consume(events <-chan crushapi.StreamEvent)
}

func AttachStream(scope context.Context, api *crushapi.Client, consumer EventConsumer, workspaceID string, lost func(scope context.Context, reason string)) error {
	events, stop, err := api.Stream(scope, workspaceID, StreamKinds...)
	if err != nil {
		return fmt.Errorf("event stream attach failed: %w", err)
	}

	go func() {
		consumer.Consume(events)
		if scope.Err() == nil {
			lost(scope, "engine event stream disconnected")
		}
	}()
	go func() {
		<-scope.Done()
		stop()
	}()
	return nil
}
