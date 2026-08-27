package main

import (
	"errors"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

// bind_permission.go -- role: Wails-bound API for approvals and questions.
//
// Requests travel UI-ward as permission:request / question:request events;
// only answers come back as bound calls.

// AnswerPermission answers an approval request. decision is one of
// "allow", "allow_session", "deny". Returns whether this call resolved
// the request (false means it was already answered elsewhere).
func (a *App) AnswerPermission(requestID string, decision string) (bool, error) {
	action := crushapi.PermissionAction(decision)
	switch action {
	case crushapi.PermissionAllow, crushapi.PermissionAllowForSession, crushapi.PermissionDeny:
	default:
		return false, errors.New("invalid decision: " + decision)
	}

	// a.perms is built in NewApp and never replaced, so only the engine-attached
	// services can be missing here.
	a.mu.RLock()
	api := a.api
	ws := a.ws
	a.mu.RUnlock()
	if api == nil || ws == nil {
		return false, errors.New("engine is not running")
	}

	req, ok := a.perms.Take(requestID)
	if !ok {
		return false, errors.New("unknown or expired permission request: " + requestID)
	}
	desc, ok := ws.Current()
	if !ok {
		return false, errors.New("no workspace attached")
	}
	return api.GrantPermission(a.ctx, desc.WorkspaceID, req, action)
}

// QuestionAnswerInput mirrors one answer inside a question batch.
type QuestionAnswerInput struct {
	QuestionID  string   `json:"request_id"`
	SelectedIDs []string `json:"selected_ids,omitempty"`
	FillInText  string   `json:"fill_in_text,omitempty"`
	Yes         *bool    `json:"yes,omitempty"`
}

// AnswerQuestion replies to an agent question batch identified by the
// question:request event ID.
func (a *App) AnswerQuestion(requestID string, answers []QuestionAnswerInput) (bool, error) {
	if len(answers) == 0 {
		return false, errors.New("empty question answers")
	}
	a.mu.RLock()
	api := a.api
	ws := a.ws
	a.mu.RUnlock()
	if api == nil || ws == nil {
		return false, errors.New("engine is not running")
	}
	desc, ok := ws.Current()
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
	return api.AnswerQuestion(a.ctx, desc.WorkspaceID, crushapi.QuestionAnswer{
		BatchRequestID: requestID,
		Responses:      responses,
	})
}
