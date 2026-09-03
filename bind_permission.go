package main

import (
	"errors"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

func (a *App) AnswerPermission(requestID string, decision string) (bool, error) {
	action := crushapi.PermissionAction(decision)
	switch action {
	case crushapi.PermissionAllow, crushapi.PermissionAllowForSession, crushapi.PermissionDeny:
	default:
		return false, errors.New("invalid decision: " + decision)
	}

	c := a.getConn()
	if c == nil || c.api == nil || c.ws == nil || c.perms == nil {
		return false, errors.New("engine is not running")
	}

	req, ok := c.perms.Take(requestID)
	if !ok {
		return false, errors.New("unknown or expired permission request: " + requestID)
	}
	desc, ok := c.ws.Current()
	if !ok {
		return false, errors.New("no workspace attached")
	}
	return c.api.GrantPermission(a.ctx, desc.WorkspaceID, req, action)
}
