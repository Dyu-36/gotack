package engineapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

type PermissionRequest struct {
	ID          string          `json:"id"`
	SessionID   string          `json:"session_id"`
	ToolCallID  string          `json:"tool_call_id"`
	ToolName    string          `json:"tool_name"`
	Description string          `json:"description"`
	Action      string          `json:"action"`
	Path        string          `json:"path"`
	Params      json.RawMessage `json:"params"`
}

func (c *Client) GrantPermission(ctx context.Context, wsID string, permission PermissionRequest, action string) error {
	body, err := json.Marshal(struct {
		Permission PermissionRequest `json:"permission"`
		Action     string            `json:"action"`
	}{permission, action})
	if err != nil {
		return err
	}
	return c.doJSON(ctx, http.MethodPost, expandPath("/v1/workspaces/{id}/permissions/grant", "id", wsID), bytes.NewReader(body), nil)
}

func (c *Client) SetPermissionsSkip(ctx context.Context, wsID string, skip bool) error {
	body, err := json.Marshal(map[string]bool{"skip": skip})
	if err != nil {
		return err
	}
	return c.doJSON(ctx, http.MethodPost, expandPath("/v1/workspaces/{id}/permissions/skip", "id", wsID), bytes.NewReader(body), nil)
}

// ReleaseWorkspace drops this client's claim without interrupting other interfaces.
func (c *Client) ReleaseWorkspace(ctx context.Context, wsID string) error {
	return c.doJSON(ctx, http.MethodDelete, expandPath("/v1/workspaces/{id}", "id", wsID)+"?client_id="+url.QueryEscape(c.clientID), nil, nil)
}

func (c *Client) AnswerQuestions(ctx context.Context, wsID string, answer any) error {
	body, err := json.Marshal(answer)
	if err != nil {
		return err
	}
	return c.doJSON(ctx, http.MethodPost, expandPath("/v1/workspaces/{id}/questions/answer", "id", wsID), bytes.NewReader(body), nil)
}
