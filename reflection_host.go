package main

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
		LoadTranscript: a.reflectionLoadTranscript, CreateSession: a.reflectionCreateSession,
		MarkReview: a.reflectionMarkReview, SendPrompt: a.reflectionSendPrompt,
		SendPromptWithBudget: a.reflectionSendPromptWithBudget,
		CancelSession: a.reflectionCancelSession, CleanupSession: a.reflectionCleanupSession,
		Preflight: a.reflectionPreflight,
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
	// The current engine endpoint returns the transcript. Bound the part we
	// extract; do not also copy every historical tool input/output into memory.
	if len(messages) > 32 {
		messages = messages[len(messages)-32:]
	}
	out := make([]reflection.Message, 0, len(messages))
	for _, message := range messages {
		if message.Role != "user" && message.Role != "assistant" {
			continue
		}
		out = append(out, reflection.Message{Role: message.Role, Text: crushapi.ExtractText(message.Parts)})
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
	return a.reflectionSendPromptWithBudget(ctx, sessionID, prompt, reflection.MaxReviewInputTokens)
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
	if !review.Memory || review.Skills {
		return errors.New("only personal memory review is enabled")
	}
	if resolveMemoryCommand() == "" {
		return errors.New("memory tool is unavailable")
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
			a.log.Debug("personal memory review not started", "err", err)
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
			a.log.Debug("memory review cleanup failed", "err", err)
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
		a.log.Debug("memory review shutdown failed", "err", err)
	}
}

func (a *App) prepareReflectionTurn(sessionID string) bool {
	if a.reflection == nil {
		a.refreshCurrentContextSnapshot()
		return false
	}
	baseCtx := a.ctx
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	ctx, cancel := context.WithTimeout(baseCtx, 500*time.Millisecond)
	if err := a.reflection.CancelForLiveTurn(ctx, sessionID); err != nil && a.log != nil {
		a.log.Debug("memory review cancellation deferred", "err", err)
	}
	cancel()
	// Learning cadence is not user memory. Restart its counter after reconnect
	// instead of loading all prior messages before accepting a foreground turn.
	if a.reflection.NeedsHydration(sessionID) {
		a.reflection.Hydrate(sessionID, 0)
	}
	a.refreshCurrentContextSnapshot()
	return true
}

func (a *App) reflectionTurnAccepted(sessionID string, ready bool) {
	if ready && a.reflection != nil {
		a.reflection.UserTurnAccepted(sessionID)
	}
}

func (a *App) assistantIteration(sessionID, messageID string, hasTools bool) {
	if a.reflection == nil || !a.reflection.AssistantIteration(sessionID, messageID, hasTools) {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.reflection.CancelReview(ctx); err != nil && a.log != nil {
			a.log.Debug("memory review iteration limit", "err", err)
		}
	}()
}

func (a *App) learningToolExecuted(sessionID, toolCallID, toolName string) {
	if a.reflection != nil {
		a.reflection.LearningToolExecuted(sessionID, toolCallID, toolName)
	}
}

func (a *App) forgetReflection(sessionID string) {
	if a.reflection != nil {
		a.reflection.Forget(sessionID)
	}
}
