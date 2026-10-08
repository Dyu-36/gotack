//go:build gotacktest

package agent

import (
	"context"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/Dyu-36/gotack/internal/agentcore/agent/tools"
	"github.com/Dyu-36/gotack/internal/agentcore/permission"
)

func TestPermissionDenialPreventsToolExecution(t *testing.T) {
	for _, grant := range []bool{false, true} {
		t.Run(map[bool]string{false: "deny", true: "allow"}[grant], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			ctx = context.WithValue(ctx, tools.SessionIDContextKey, "session")
			service := permission.NewPermissionService(t.TempDir(), false, nil)
			requests := service.Subscribe(ctx)
			ran := false
			tool := fantasy.NewAgentTool("edit", "edit a file", func(context.Context, map[string]any, fantasy.ToolCall) (fantasy.ToolResponse, error) {
				ran = true
				return fantasy.NewTextResponse("edited"), nil
			})
			guard := &permissionTool{AgentTool: tool, permissions: service, workingDir: t.TempDir()}
			done := make(chan fantasy.ToolResponse, 1)
			go func() {
				response, err := guard.Run(ctx, fantasy.ToolCall{ID: "call", Input: `{"file_path":"file.txt"}`})
				if err != nil {
					t.Error(err)
				}
				done <- response
			}()
			select {
			case request := <-requests:
				if request.Payload.SessionID != "session" || request.Payload.ToolCallID != "call" {
					t.Errorf("wrong permission: %+v", request.Payload)
				}
				if grant {
					service.Grant(request.Payload)
				} else {
					service.Deny(request.Payload)
				}
			case <-ctx.Done():
				t.Fatal("tool never requested permission")
			}
			select {
			case result := <-done:
				if ran != grant || result.IsError == grant {
					t.Fatalf("ran=%v, result=%+v, grant=%v", ran, result, grant)
				}
			case <-ctx.Done():
				t.Fatal("tool did not finish")
			}
		})
	}
}
