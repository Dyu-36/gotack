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

type QuestionAnswerInput struct {
	QuestionID  string   `json:"request_id"`
	SelectedIDs []string `json:"selected_ids,omitempty"`
	FillInText  string   `json:"fill_in_text,omitempty"`
	Yes         *bool    `json:"yes,omitempty"`
}

func (a *App) AnswerQuestion(requestID string, answers []QuestionAnswerInput) (bool, error) {
	if len(answers) == 0 {
		return false, errors.New("empty question answers")
	}
	c := a.getConn()
	if c == nil || c.api == nil || c.ws == nil {
		return false, errors.New("engine is not running")
	}
	desc, ok := c.ws.Current()
	if !ok {
		return false, errors.New("no workspace attached")
	}

	responses := make([]crushapi.QuestionResponse, len(answers))
	for i, ans := range answers {
		responses[i] = crushapi.QuestionResponse{
			QuestionID:  ans.QuestionID,
			SelectedIDs: ans.SelectedIDs,
			FillInText:  ans.FillInText,
			Yes:         ans.Yes,
		}
	}
	return c.api.AnswerQuestion(a.ctx, desc.WorkspaceID, crushapi.QuestionAnswer{
		BatchRequestID: requestID,
		Responses:      responses,
	})
}
