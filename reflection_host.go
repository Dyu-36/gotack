package main

// reflection_host.go -- role: adapt Hermes' bounded background review to the
// Crush REST + SSE boundary; Crush remains the sole turn executor.

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/guard"
	"github.com/Dyu-36/gotack/internal/reflection"
)

func (a *App) startReflection() {
	a.reflection = reflection.New(reflection.Runtime{
		LoadTranscript:       a.reflectionLoadTranscript,
		CreateSession:        a.reflectionCreateSession,
		MarkReview:           a.reflectionMarkReview,
		SendPrompt:           a.reflectionSendPrompt,
		SendPromptWithBudget: a.reflectionSendPromptWithBudget,
		CancelSession:        a.reflectionCancelSession,
		CleanupSession:       a.reflectionCleanupSession,
		Preflight:            a.reflectionPreflight,
	}, a.log)
}

func (a *App) reflectionLoadTranscript(ctx context.Context, sourceID string) ([]reflection.Message, error) {
	svc, err := a.services()
	if err != nil {
		return nil, err
	}
	messages, err := svc.sess.Messages(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	out := make([]reflection.Message, 0, len(messages))
	for _, message := range messages {
		parts := crushapi.ExtractParts(message.Parts)
		tools := make([]string, 0, len(parts.ToolCalls))
		for _, call := range parts.ToolCalls {
			if call.Name != "" {
				tools = append(tools, call.Name)
			}
		}
		results := make([]reflection.ToolResult, 0, len(parts.ToolResults))
		for _, result := range parts.ToolResults {
			results = append(results, reflection.ToolResult{
				Name: result.Name, Content: result.Content, IsError: result.IsError,
			})
		}
		out = append(out, reflection.Message{
			Role: message.Role, Text: parts.Text, Tools: tools, Results: results,
		})
	}
	return out, nil
}

func (a *App) reflectionCreateSession(ctx context.Context, title string) (string, error) {
	svc, err := a.services()
	if err != nil {
		return "", err
	}
	sess, err := svc.sess.Create(ctx, title)
	if err != nil {
		return "", err
	}
	return sess.ID, nil
}

func (a *App) reflectionMarkReview(_ context.Context, sessionID string) error {
	return guard.MarkReviewSession(filepath.Join(appconfig.Dir(), guard.ReviewRosterFileName), sessionID)
}

func (a *App) reflectionSendPrompt(ctx context.Context, sessionID, prompt string) (string, error) {
	svc, err := a.services()
	if err != nil {
		return "", err
	}
	return svc.sess.Send(ctx, sessionID, prompt)
}

func (a *App) reflectionSendPromptWithBudget(ctx context.Context, sessionID, prompt string, maxInputTokens int64) (string, error) {
	svc, err := a.services()
	if err != nil {
		return "", err
	}
	return svc.sess.SendWithInputBudget(ctx, sessionID, prompt, maxInputTokens)
}

func (a *App) reflectionCancelSession(ctx context.Context, sessionID string) error {
	svc, err := a.services()
	if err != nil {
		return err
	}
	return svc.sess.Cancel(ctx, sessionID)
}

func (a *App) reflectionCleanupSession(ctx context.Context, sessionID string) error {
	rosterErr := guard.UnmarkReviewSession(filepath.Join(appconfig.Dir(), guard.ReviewRosterFileName), sessionID)
	svc, err := a.services()
	if err != nil {
		return errors.Join(rosterErr, err)
	}
	return errors.Join(rosterErr, svc.sess.Delete(ctx, sessionID))
}

func (a *App) reflectionPreflight(_ context.Context, review reflection.Review) error {
	if a.cfg == nil || strings.TrimSpace(a.cfg.Model) == "" {
		return errors.New("no model configured")
	}
	if review.Memory && resolveMemoryCommand() == "" {
		return errors.New("memory tool is unavailable")
	}
	if review.Skills && resolveSkillsCommand() == "" {
		return errors.New("skills tool is unavailable")
	}
	return nil
}

func (a *App) triggerReflection(sourceID string, review reflection.Review) {
	if a.reflection == nil || !review.Any() {
		return
	}
	baseCtx := a.ctx
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	go func() {
		if err := a.reflection.Fire(baseCtx, sourceID, review); err != nil && a.log != nil {
			a.log.Debug("background review not started", "session", sourceID, "err", err)
		}
	}()
}

func (a *App) cleanupReflection(sessionID string) {
	if sessionID == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := a.reflectionCleanupSession(ctx, sessionID); err != nil && a.log != nil {
			a.log.Debug("background review cleanup failed", "session", sessionID, "err", err)
		}
	}()
}

func (a *App) stopReflection(ctx context.Context) {
	if a.reflection == nil {
		return
	}
	sessionID, cancelErr := a.reflection.Stop(ctx)
	var cleanupErr error
	if sessionID != "" {
		cleanupErr = a.reflectionCleanupSession(ctx, sessionID)
	}
	if err := errors.Join(cancelErr, cleanupErr); err != nil && a.log != nil {
		a.log.Debug("background review shutdown cleanup failed", "session", sessionID, "err", err)
	}
}

func (a *App) prepareReflectionTurn(sessionID string) bool {
	if a.reflection == nil {
		return false
	}
	baseCtx := a.ctx
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	ctx, cancel := context.WithTimeout(baseCtx, 5*time.Second)
	if err := a.reflection.CancelForLiveTurn(ctx, sessionID); err != nil && a.log != nil {
		a.log.Debug("background review cancellation failed", "session", sessionID, "err", err)
	}
	cancel()
	if !a.reflection.NeedsHydration(sessionID) {
		return true
	}
	svc, err := a.services()
	if err != nil {
		return false
	}
	messages, err := svc.sess.Messages(baseCtx, sessionID)
	if err != nil {
		if a.log != nil {
			a.log.Debug("learning cadence hydration deferred", "session", sessionID, "err", err)
		}
		return false
	}
	userTurns := 0
	for _, message := range messages {
		if strings.EqualFold(message.Role, "user") {
			userTurns++
		}
	}
	a.reflection.Hydrate(sessionID, userTurns)
	return true
}

func (a *App) reflectionTurnAccepted(sessionID string, ready bool) {
	if ready && a.reflection != nil {
		a.reflection.UserTurnAccepted(sessionID)
	}
}

func (a *App) AssistantIteration(sessionID, messageID string, hasTools bool) {
	if a.reflection == nil || !a.reflection.AssistantIteration(sessionID, messageID, hasTools) {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.reflection.CancelReview(ctx); err != nil && a.log != nil {
			a.log.Debug("background review iteration limit cancellation failed", "err", err)
		}
	}()
}

func (a *App) LearningToolExecuted(sessionID, toolCallID, toolName string) {
	if a.reflection != nil {
		a.reflection.LearningToolExecuted(sessionID, toolCallID, toolName)
	}
}

func (a *App) forgetReflection(sessionID string) {
	if a.reflection != nil {
		a.reflection.Forget(sessionID)
	}
}
