package enginelink

import (
	"context"
	"errors"
	"fmt"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

// Typed errors for the stream machinery. The host returns them from bound
// methods verbatim; the UI surfaces the text, so the wording is part of the
// desktop contract.
var (
	// ErrWorkspaceIDRequired rejects a stream attach with no workspace.
	ErrWorkspaceIDRequired = errors.New("workspace id is required for event stream")
	// ErrNoConnection rejects stream work before any engine connection exists.
	ErrNoConnection = errors.New("engine connection unavailable")
	// ErrTransportNotWired rejects a stream attach while services are missing.
	ErrTransportNotWired = errors.New("event stream unavailable: transport not wired")
)

// StreamKinds is the SSE envelope allow-list Gotack forwards to the UI.
// Every kind maps to a handler in internal/uievents; adding one here without
// a handler would deliver events the UI ignores.
var StreamKinds = []string{
	"message", "run_complete", "permission_request", "question_batch_request", "file",
}

// EventConsumer drains decoded SSE events. *uievents.Forwarder satisfies it;
// the host wires the concrete consumer, so this package never imports
// uievents.
type EventConsumer interface {
	Consume(events <-chan crushapi.StreamEvent)
}

// AttachStream subscribes consumer to the workspace SSE channel for the
// lifetime of scope. Crush considers the client attached only while this
// stream is live, so callers install it synchronously before relying on
// session-presence calls.
//
// Two goroutines outlive the call: one forwards events and reports an
// unexpected close through lost(scope, reason), and one stops the stream
// when scope is cancelled. A close caused by cancellation never calls lost,
// because an intentional disconnect is not a transport failure.
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
