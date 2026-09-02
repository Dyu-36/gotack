package reflection

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const (
	MemoryInterval      = 10
	SkillInterval       = 10
	MaxReviewIterations = 16

	MaxReviewInputTokens int64 = 600_000
	defaultFireTimeout         = 30 * time.Second
)

type Review struct {
	Memory bool
	Skills bool
}

func (r Review) Any() bool { return r.Memory || r.Skills }

type Runtime struct {
	LoadTranscript func(context.Context, string) ([]Message, error)
	CreateSession  func(context.Context, string) (string, error)
	MarkReview     func(context.Context, string) error
	SendPrompt     func(context.Context, string, string) (string, error)

	SendPromptWithBudget func(context.Context, string, string, int64) (string, error)
	CancelSession        func(context.Context, string) error
	CleanupSession       func(context.Context, string) error
	Preflight            func(context.Context, Review) error
}

type sessionState struct {
	hydrated             bool
	turnsSinceMemory     int
	iterationsSinceSkill int
	memoryDue            bool
}

type iterationSeen struct {
	hasTools bool
}

type Tracker struct {
	rt  Runtime
	log *slog.Logger

	mu               sync.Mutex
	sessions         map[string]*sessionState
	seenIterations   map[string]iterationSeen
	seenLearningCall map[string]struct{}
	inflight         bool
	reviewSessionID  string
	reviewIterations int
	launchCancel     context.CancelFunc
	fireTimeout      time.Duration
}

func New(rt Runtime, log *slog.Logger) *Tracker {
	if log == nil {
		log = slog.Default()
	}
	return &Tracker{
		rt: rt, log: log,
		sessions:         make(map[string]*sessionState),
		seenIterations:   make(map[string]iterationSeen),
		seenLearningCall: make(map[string]struct{}),
		fireTimeout:      defaultFireTimeout,
	}
}

func (t *Tracker) NeedsHydration(sessionID string) bool {
	if sessionID == "" {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	state := t.sessions[sessionID]
	return state == nil || !state.hydrated
}

func (t *Tracker) Hydrate(sessionID string, priorUserTurns int) {
	if sessionID == "" {
		return
	}
	if priorUserTurns < 0 {
		priorUserTurns = 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	state := t.sessions[sessionID]
	if state == nil {
		state = &sessionState{}
		t.sessions[sessionID] = state
	}
	if state.hydrated {
		return
	}
	state.turnsSinceMemory = priorUserTurns % MemoryInterval
	state.hydrated = true
}

func (t *Tracker) UserTurnAccepted(sessionID string) {
	if sessionID == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	state := t.sessions[sessionID]
	if state == nil {
		state = &sessionState{hydrated: true}
		t.sessions[sessionID] = state
	}
	state.hydrated = true
	state.turnsSinceMemory++
	if state.turnsSinceMemory >= MemoryInterval {
		state.turnsSinceMemory = 0
		state.memoryDue = true
	}
}

func (t *Tracker) AssistantIteration(sessionID, messageID string, hasTools bool) bool {
	if sessionID == "" || messageID == "" {
		return false
	}
	key := sessionID + "\x00" + messageID
	t.mu.Lock()
	defer t.mu.Unlock()

	seen, duplicate := t.seenIterations[key]
	if duplicate {
		newTools := hasTools && !seen.hasTools
		seen.hasTools = seen.hasTools || hasTools
		t.seenIterations[key] = seen
		if sessionID == t.reviewSessionID {
			return newTools && t.reviewIterations >= MaxReviewIterations
		}
		return false
	}

	t.seenIterations[key] = iterationSeen{hasTools: hasTools}
	if sessionID == t.reviewSessionID {
		t.reviewIterations++
		return t.reviewIterations >= MaxReviewIterations && hasTools
	}
	state := t.sessions[sessionID]
	if state == nil {
		state = &sessionState{}
		t.sessions[sessionID] = state
	}
	state.iterationsSinceSkill++
	return false
}

func (t *Tracker) LearningToolExecuted(sessionID, toolCallID, toolName string) {
	if sessionID == "" || toolCallID == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if sessionID == t.reviewSessionID {
		return
	}
	key := sessionID + "\x00" + toolCallID
	if _, duplicate := t.seenLearningCall[key]; duplicate {
		return
	}
	t.seenLearningCall[key] = struct{}{}
	state := t.sessions[sessionID]
	if state == nil {
		state = &sessionState{}
		t.sessions[sessionID] = state
	}
	switch toolName {
	case "memory", "mcp_gotack-memory_memory":
		state.turnsSinceMemory = 0
	case "skill_manage", "mcp_gotack-skills_skill_manage":
		state.iterationsSinceSkill = 0
	}
}

func (t *Tracker) RunDone(sessionID, finalText, runErr string, cancelled bool) (Review, string) {
	if sessionID == "" {
		return Review{}, ""
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if sessionID == t.reviewSessionID {
		cleanupID := t.reviewSessionID
		t.clearReviewLocked()
		return Review{}, cleanupID
	}
	t.clearSeenSessionLocked(sessionID)
	state := t.sessions[sessionID]
	if state == nil {
		return Review{}, ""
	}
	review := Review{Memory: state.hydrated && state.memoryDue}
	state.memoryDue = false
	if state.iterationsSinceSkill >= SkillInterval {
		review.Skills = true
		state.iterationsSinceSkill = 0
	}
	if runErr != "" || cancelled || strings.TrimSpace(finalText) == "" {
		return Review{}, ""
	}
	return review, ""
}

func (t *Tracker) Fire(ctx context.Context, sourceSessionID string, review Review) error {
	if strings.TrimSpace(sourceSessionID) == "" {
		return errors.New("reflection: source session id is required")
	}
	if !review.Any() {
		return errors.New("reflection: review target is required")
	}
	if t.rt.Preflight != nil {
		if err := t.rt.Preflight(ctx, review); err != nil {
			return fmt.Errorf("reflection preflight: %w", err)
		}
	}

	t.mu.Lock()
	if t.inflight {
		t.mu.Unlock()
		return errors.New("reflection: a review is already in flight")
	}
	fireCtx, cancel := context.WithTimeout(ctx, t.fireTimeout)
	t.inflight = true
	t.launchCancel = cancel
	t.mu.Unlock()

	var reviewSessionID string
	fail := func(err error) error {
		cancel()
		t.mu.Lock()
		t.clearReviewLocked()
		t.mu.Unlock()
		if reviewSessionID != "" && t.rt.CleanupSession != nil {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = t.rt.CleanupSession(cleanupCtx, reviewSessionID)
			cleanupCancel()
		}
		t.log.Warn("reflection: launch failed", "source", sourceSessionID, "err", err)
		return fmt.Errorf("reflection launch: %w", err)
	}
	if t.rt.LoadTranscript == nil || t.rt.CreateSession == nil || t.rt.MarkReview == nil ||
		(t.rt.SendPrompt == nil && t.rt.SendPromptWithBudget == nil) {
		return fail(errors.New("runtime is incomplete"))
	}
	messages, err := t.rt.LoadTranscript(fireCtx, sourceSessionID)
	if err != nil {
		return fail(err)
	}
	reviewSessionID, err = t.rt.CreateSession(fireCtx, "Background review")
	if err != nil {
		return fail(err)
	}
	if err = t.rt.MarkReview(fireCtx, reviewSessionID); err != nil {
		return fail(err)
	}
	t.mu.Lock()
	if fireCtx.Err() != nil {
		t.mu.Unlock()
		return fail(fireCtx.Err())
	}
	t.reviewSessionID = reviewSessionID
	t.reviewIterations = 0
	t.mu.Unlock()
	var sendErr error
	if t.rt.SendPromptWithBudget != nil {
		_, sendErr = t.rt.SendPromptWithBudget(fireCtx, reviewSessionID, Prompt(messages, review), MaxReviewInputTokens)
	} else {
		_, sendErr = t.rt.SendPrompt(fireCtx, reviewSessionID, Prompt(messages, review))
	}
	if sendErr != nil {
		return fail(sendErr)
	}
	return nil
}

func (t *Tracker) CancelForLiveTurn(ctx context.Context, liveSessionID string) error {
	t.mu.Lock()
	if liveSessionID != "" && liveSessionID == t.reviewSessionID {
		t.mu.Unlock()
		return nil
	}
	if t.launchCancel != nil {
		t.launchCancel()
	}
	reviewSessionID := t.reviewSessionID
	t.mu.Unlock()
	if reviewSessionID == "" || t.rt.CancelSession == nil {
		return nil
	}
	return t.rt.CancelSession(ctx, reviewSessionID)
}

func (t *Tracker) CancelReview(ctx context.Context) error {
	t.mu.Lock()
	reviewSessionID := t.reviewSessionID
	t.mu.Unlock()
	if reviewSessionID == "" || t.rt.CancelSession == nil {
		return nil
	}
	return t.rt.CancelSession(ctx, reviewSessionID)
}

func (t *Tracker) Stop(ctx context.Context) (string, error) {
	t.mu.Lock()
	reviewSessionID := t.reviewSessionID
	t.clearReviewLocked()
	t.mu.Unlock()
	if reviewSessionID == "" || t.rt.CancelSession == nil {
		return reviewSessionID, nil
	}
	return reviewSessionID, t.rt.CancelSession(ctx, reviewSessionID)
}

func (t *Tracker) Forget(sessionID string) {
	if sessionID == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.sessions, sessionID)
	t.clearSeenSessionLocked(sessionID)
}

func (t *Tracker) clearSeenSessionLocked(sessionID string) {
	prefix := sessionID + "\x00"
	for key := range t.seenIterations {
		if strings.HasPrefix(key, prefix) {
			delete(t.seenIterations, key)
		}
	}
	for key := range t.seenLearningCall {
		if strings.HasPrefix(key, prefix) {
			delete(t.seenLearningCall, key)
		}
	}
}

func (t *Tracker) clearReviewLocked() {
	if t.launchCancel != nil {
		t.launchCancel()
	}
	if t.reviewSessionID != "" {
		t.clearSeenSessionLocked(t.reviewSessionID)
	}
	t.inflight = false
	t.reviewSessionID = ""
	t.reviewIterations = 0
	t.launchCancel = nil
}
