package agent

import (
	"context"
	"encoding/json"
	"path/filepath"

	"charm.land/fantasy"
	"github.com/Dyu-36/gotack/internal/agentcore/agent/tools"
	"github.com/Dyu-36/gotack/internal/agentcore/permission"
)

type permissionTool struct {
	fantasy.AgentTool
	permissions permission.Service
	workingDir  string
}

func (t *permissionTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	path := t.workingDir
	var params map[string]any
	if err := json.Unmarshal([]byte(call.Input), &params); err == nil {
		if file, ok := params["file_path"].(string); ok && file != "" {
			path = file
			if !filepath.IsAbs(path) {
				path = filepath.Join(t.workingDir, path)
			}
		}
	}
	granted, err := t.permissions.Request(ctx, permission.CreatePermissionRequest{
		SessionID: tools.GetSessionFromContext(ctx), ToolCallID: call.ID,
		ToolName: t.Info().Name, Action: "execute", Path: path,
		Description: "Execute " + t.Info().Name, Params: params,
	})
	if err != nil {
		return fantasy.ToolResponse{}, err
	}
	if !granted {
		return fantasy.NewTextErrorResponse("Tool execution denied by the user"), nil
	}
	return t.AgentTool.Run(ctx, call)
}

func (c *coordinator) withPermissions(tool fantasy.AgentTool) fantasy.AgentTool {
	if c.permissions == nil {
		return tool
	}
	// These built-ins inspect files. Mutation, shell commands, and extension
	// tools require permission; the shared service implements explicit YOLO
	// and session-scoped grants.
	switch tool.Info().Name {
	case tools.ViewToolName, tools.GrepToolName, tools.GlobToolName:
		return tool
	}
	return &permissionTool{AgentTool: tool, permissions: c.permissions, workingDir: c.cfg.WorkingDir()}
}
