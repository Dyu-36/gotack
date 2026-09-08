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
	MemoryInterval      = 15
	MaxReviewIterations = 3
	// Cumulative input across the review run, not a context window per call.
	MaxReviewInputTokens int64 = 32_000
	defaultFireTimeout         = 30 * time.Second
	defaultIdleDelay           = 5 * time.Second
)

type Review struct {
	Memory bool
	Skills bool // Compatibility with callers; automatic skill review is disabled.
}

func (r Review) Any() bool { return r.Memory }

type Runtime struct {
	LoadTranscript       func(context.Context, string) ([]Message, error)
	CreateSession        func(context.Context, string) (string, error)
	MarkReview           func(context.Context, string) error
	SendPrompt           func(context.Context, string, string) (string, error)
	SendPromptWithBudget func(context.Context, string, string, int64) (string, error)
	CancelSession        func(context.Context, string) error
	CleanupSession       func(context.Context, string) error
	Preflight            func(context.Context, Review) error
}

type sessionState struct {
	hydrated         bool
	turnsSinceMemory int
	memoryDue        bool
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
	launchID         uint64
	fireTimeout      time.Duration
	idleDelay        time.Duration
}

func New(rt Runtime, log *slog.Logger) *Tracker {
	if log == nil {
		log = slog.Default()
	}
	return &Tracker{
		rt: rt, log: log, sessions: make(map[string]*sessionState),
		seenIterations: make(map[string]iterationSeen), seenLearningCall: make(map[string]struct{}),
		fireTimeout: defaultFireTimeout, idleDelay: defaultIdleDelay,
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
	if !state.hydrated {
		state.turnsSinceMemory = priorUserTurns % MemoryInterval
		state.hydrated = true
	}
}

func (t *Tracker) UserTurnAccepted(sessionID string) {
	if sessionID == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	state := t.sessions[sessionID]
	if state == nil {
		state = &sessionState{}
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
	t.mu.Lock()
	defer t.mu.Unlock()
	if sessionID != t.reviewSessionID {
		return false
	}
	key := sessionID + "\x00" + messageID
	seen, duplicate := t.seenIterations[key]
	if duplicate {
		newTools := hasTools && !seen.hasTools
		seen.hasTools = seen.hasTools || hasTools
		t.seenIterations[key] = seen
		return newTools && t.reviewIterations >= MaxReviewIterations
	}
	t.seenIterations[key] = iterationSeen{hasTools: hasTools}
	t.reviewIterations++
	return t.reviewIterations >= MaxReviewIterations && hasTools
}

func (t *Tracker) LearningToolExecuted(sessionID, toolCallID, toolName string) {
	if sessionID == "" || toolCallID == "" {
		return
	}
	if toolName != "memory" && toolName != "mcp_gotack-memory_memory" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if sessionID == t.reviewSessionID {
		return
	}
	key := sessionID + "\x00" + toolCallID
	if _, exists := t.seenLearningCall[key]; exists {
		return
	}
	t.seenLearningCall[key] = struct{}{}
	if state := t.sessions[sessionID]; state != nil {
		state.turnsSinceMemory = 0
		state.memoryDue = false
	}
}

func (t *Tracker) RunDone(sessionID, finalText, runErr string, cancelled bool) (Review, string) {
	if sessionID == "" {
		return Review{}, ""
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if sessionID == t.reviewSessionID {
		id := t.reviewSessionID
		t.clearReviewLocked()
		return Review{}, id
	}
	t.clearSeenSessionLocked(sessionID)
	state := t.sessions[sessionID]
	if state == nil {
		return Review{}, ""
	}
	review := Review{Memory: state.hydrated && state.memoryDue}
	state.memoryDue = false
	if runErr != "" || cancelled || strings.TrimSpace(finalText) == "" {
		return Review{}, ""
	}
	return review, ""
}

func (t *Tracker) Fire(ctx context.Context, sourceSessionID string, review Review) error {
	if strings.TrimSpace(sourceSessionID) == "" {
		return errors.New("reflection: source session id is required")
	}
	if !review.Memory || review.Skills {
		return errors.New("reflection: only personal memory review is supported")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	t.mu.Lock()
	if t.inflight {
		t.mu.Unlock()
		return errors.New("reflection: a review is already in flight")
	}
	fireCtx, cancel := context.WithTimeout(ctx, t.fireTimeout)
	t.launchID++
	launchID := t.launchID
	t.inflight = true
	t.launchCancel = cancel
	t.mu.Unlock()

	var reviewSessionID string
	fail := func(err error) error {
		cancel()
		t.mu.Lock()
		// A cancelled old launcher must not clear a newer review after Stop.
		if t.launchID == launchID {
			t.clearReviewLocked()
		}
		t.mu.Unlock()
		if reviewSessionID != "" && t.rt.CleanupSession != nil {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = t.rt.CleanupSession(cleanupCtx, reviewSessionID)
			cleanupCancel()
		}
		return fmt.Errorf("reflection launch: %w", err)
	}
	// Foreground input cancels this wait before any model request is made.
	timer := time.NewTimer(t.idleDelay)
	defer timer.Stop()
	select {
	case <-fireCtx.Done():
		return fail(fireCtx.Err())
	case <-timer.C:
	}
	if t.rt.Preflight != nil {
		if err := t.rt.Preflight(fireCtx, review); err != nil {
			return fail(err)
		}
	}
	if t.rt.LoadTranscript == nil || t.rt.CreateSession == nil || t.rt.MarkReview == nil || t.rt.SendPromptWithBudget == nil {
		return fail(errors.New("bounded memory review runtime is incomplete"))
	}
	messages, err := t.rt.LoadTranscript(fireCtx, sourceSessionID)
	if err != nil {
		return fail(err)
	}
	reviewSessionID, err = t.rt.CreateSession(fireCtx, "Personal memory review")
	if err != nil {
		return fail(err)
	}
	if err := t.rt.MarkReview(fireCtx, reviewSessionID); err != nil {
		return fail(err)
	}
	t.mu.Lock()
	if fireCtx.Err() != nil || t.launchID != launchID {
		t.mu.Unlock()
		return fail(context.Canceled)
	}
	t.reviewSessionID = reviewSessionID
	t.reviewIterations = 0
	t.mu.Unlock()
	_, err = t.rt.SendPromptWithBudget(fireCtx, reviewSessionID, Prompt(messages, review), MaxReviewInputTokens)
	if err != nil {
		return fail(err)
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
	id := t.reviewSessionID
	t.mu.Unlock()
	if id == "" || t.rt.CancelSession == nil {
		return nil
	}
	return t.rt.CancelSession(ctx, id)
}

func (t *Tracker) CancelReview(ctx context.Context) error {
	t.mu.Lock()
	id := t.reviewSessionID
	t.mu.Unlock()
	if id == "" || t.rt.CancelSession == nil {
		return nil
	}
	return t.rt.CancelSession(ctx, id)
}

func (t *Tracker) Stop(ctx context.Context) (string, error) {
	t.mu.Lock()
	id := t.reviewSessionID
	t.launchID++
	t.clearReviewLocked()
	t.mu.Unlock()
	if id == "" || t.rt.CancelSession == nil {
		return id, nil
	}
	return id, t.rt.CancelSession(ctx, id)
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
