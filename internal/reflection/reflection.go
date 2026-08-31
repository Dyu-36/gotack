package reflection

// reflection.go -- role: the Phase 6 learning-loop gate and bounded-run
// launcher.
//
// Semantics (plan 6.1-6.2, contract docs/contracts/gotack-reflection.md):
//   - The only post-run signal is the run_complete SSE payload; Crush exposes
//     no post-run hook. The host feeds those completions here through
//     SessionDone, and session deletions through SessionEnded.
//   - Not every turn deserves reflection: the turn-count gate and the
//     session-end gate decide, and a per-hour budget is the second stop.
//   - Recursion guard: reflection sessions are tagged at creation and their
//     own completions are never counted, so the loop cannot feed itself.
//   - Proposals route through decision 0003: the fired prompt allows memory
//     writes only through the gotack-memory MCP tool, and the guard denies
//     every other write path into the context directory.
//   - One firing submits ONE bounded agent run through the Runtime seam, the
//     same shape internal/schedule fires through (ADR 0001): the host never
//     runs agent logic itself.

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
	// TitlePrefix tags every reflection session by title. The tracker also
	// remembers the ids it launched, but the prefix keeps reflection runs
	// recognisable to humans in the session list.
	TitlePrefix = "Reflection: "

	// DefaultTurnThreshold is how many successful completed turns a session
	// needs before it becomes eligible for reflection. Unconditional
	// reflection per run would double token cost (plan 6.2), so the gate
	// opens only every N turns.
	DefaultTurnThreshold = 8

	// DefaultHourlyBudget is the second stop: at most this many reflection
	// runs per sliding one-hour window, regardless of how many gates open.
	DefaultHourlyBudget = 1

	// budgetWindow is the sliding window the hourly budget counts inside.
	budgetWindow = time.Hour

	// defaultFireTimeout bounds one launch sequence (create, mark, send).
	defaultFireTimeout = 30 * time.Second
)

// Runtime is the desktop seam the tracker fires through. The host supplies
// implementations over internal/crushapi and the guard roster; Crush stays
// the only executor of agent logic.
type Runtime struct {
	CreateSession  func(ctx context.Context, title string) (sessionID string, err error)
	MarkUnattended func(ctx context.Context, sessionID string) error
	SendPrompt     func(ctx context.Context, sessionID, prompt string) (runID string, err error)
	// Preflight skips a firing instead of burning a failed run (for example
	// when no provider/model is configured). Its error is surfaced, never
	// counted against the budget.
	Preflight func(ctx context.Context) error
}

// Tracker owns the reflection gates, the hourly budget and the recursion
// guard. All state is in-memory by design: the loop persists nothing, so
// removing the feature leaves no state behind.
type Tracker struct {
	rt  Runtime
	log *slog.Logger

	now           func() time.Time
	turnThreshold int
	hourlyBudget  int
	fireTimeout   time.Duration

	mu                 sync.Mutex
	turns              map[string]int  // successful completed turns per source session
	reflected          map[string]bool // session-end trigger fires once per session
	reflectionSessions map[string]bool // recursion guard: sessions this tracker launched
	fires              []time.Time     // launch timestamps inside the budget window
	inflight           bool
}

// New builds a Tracker over the Runtime seam. Constructing performs no I/O
// and starts no goroutine; the host calls Fire when a gate opens.
func New(rt Runtime, log *slog.Logger) *Tracker {
	if log == nil {
		log = slog.Default()
	}
	return &Tracker{
		rt:                 rt,
		log:                log,
		now:                time.Now,
		turnThreshold:      DefaultTurnThreshold,
		hourlyBudget:       DefaultHourlyBudget,
		fireTimeout:        defaultFireTimeout,
		turns:              make(map[string]int),
		reflected:          make(map[string]bool),
		reflectionSessions: make(map[string]bool),
	}
}

// SessionDone consumes one run_complete payload. It returns whether the
// session just crossed the turn-count gate and a reflection run should be
// started. Completions of tagged reflection sessions are the recursion
// guard: they are never counted, and they release the in-flight claim so
// the loop never feeds itself. Errored and cancelled runs never count.
func (t *Tracker) SessionDone(sessionID, runErr string, cancelled bool) bool {
	if sessionID == "" {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.reflectionSessions[sessionID] {
		t.inflight = false
		return false
	}
	if runErr != "" || cancelled {
		return false
	}
	t.turns[sessionID]++
	return t.turns[sessionID] >= t.turnThreshold
}

// SessionEnded is the session-end gate: it opens once per session, and only
// when the session saw at least one successful turn under this host run.
// The host calls it before deleting the session so the reflection run still
// finds the source conversation.
func (t *Tracker) SessionEnded(sessionID string) bool {
	if sessionID == "" {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.reflectionSessions[sessionID] || t.reflected[sessionID] {
		return false
	}
	if t.turns[sessionID] < 1 {
		return false
	}
	t.reflected[sessionID] = true
	return true
}

// Fire launches one bounded reflection run about sourceSessionID. The call
// order is pinned by tests: preflight, create session, mark it unattended,
// then submit the prompt. A failed mark aborts the firing: an unmarked
// unattended session could hang on an approval prompt nobody can answer
// (ADR 0002). A failed launch releases the in-flight claim and consumes no
// budget, so a later gate decision can try again.
func (t *Tracker) Fire(ctx context.Context, sourceSessionID string) error {
	if strings.TrimSpace(sourceSessionID) == "" {
		return errors.New("reflection: source session id is required")
	}
	if t.rt.Preflight != nil {
		if err := t.rt.Preflight(ctx); err != nil {
			return fmt.Errorf("reflection preflight: %w", err)
		}
	}

	t.mu.Lock()
	if t.inflight {
		t.mu.Unlock()
		return errors.New("reflection: a run is already in flight")
	}
	now := t.now()
	if !t.budgetAllowsLocked(now) {
		t.mu.Unlock()
		return errors.New("reflection: hourly budget exhausted")
	}
	t.inflight = true
	t.mu.Unlock()

	fctx, cancel := context.WithTimeout(ctx, t.fireTimeout)
	defer cancel()

	sessionID, err := t.rt.CreateSession(fctx, TitlePrefix+sourceSessionID)
	if err == nil {
		err = t.rt.MarkUnattended(fctx, sessionID)
	}
	if err == nil {
		_, err = t.rt.SendPrompt(fctx, sessionID, PromptFor(sourceSessionID))
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if err != nil {
		t.inflight = false
		t.log.Warn("reflection: launch failed", "source", sourceSessionID, "err", err)
		return fmt.Errorf("reflection launch: %w", err)
	}
	t.reflectionSessions[sessionID] = true
	t.fires = append(pruneFires(t.fires, now), now)
	delete(t.turns, sourceSessionID)
	return nil
}

// PromptFor builds the reflection prompt. Decision 0003: memory entries may
// only be proposed through the gotack-memory tool; every other write path
// into the context directory is forbidden here and denied by the guard.
func PromptFor(sourceSessionID string) string {
	return "You are running an automated Gotack reflection pass about session " + sourceSessionID + ".\n\n" +
		"Goal: decide whether that session revealed durable, reusable knowledge worth keeping, and store it if so.\n\n" +
		"Steps:\n" +
		"1. Review what happened in session " + sourceSessionID + ". If the session_search tool is available, use it to look up that session's messages.\n" +
		"2. Identify durable facts only: stable user preferences, project constraints, and decisions that will matter in future sessions. Skip transient task details and anything already stored.\n" +
		"3. Read the current memory through the gotack-memory `memory tool` (action \"view\"), then propose entries only through that same memory tool (action \"add\" or \"replace\"), one short self-contained section per fact.\n" +
		"4. If nothing durable emerged, store nothing and finish with one sentence.\n\n" +
		"Hard rules for this run:\n" +
		"- Never write context files directly and never use write, edit, or shell tools to touch MEMORY.md or USER.md; the gotack-memory memory tool is the only permitted write path.\n" +
		"- Do not perform any other work: no file edits, no commands, no installs.\n" +
		"- Keep entries short; the memory tool enforces size caps and rejects oversized entries."
}

// budgetAllowsLocked reports whether one more reflection run may launch now.
// Callers hold t.mu.
func (t *Tracker) budgetAllowsLocked(now time.Time) bool {
	since := now.Add(-budgetWindow)
	count := 0
	for _, f := range t.fires {
		if f.After(since) {
			count++
		}
	}
	return count < t.hourlyBudget
}

// pruneFires drops fire records outside the budget window. Reuses the
// backing array: callers must treat the input as consumed.
func pruneFires(fires []time.Time, now time.Time) []time.Time {
	since := now.Add(-budgetWindow)
	out := fires[:0]
	for _, f := range fires {
		if f.After(since) {
			out = append(out, f)
		}
	}
	return out
}

// turnCount reports the recorded completed-turn count for sessionID. Test
// seam only.
func (t *Tracker) turnCount(sessionID string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.turns[sessionID]
}

// tagReflectionSession marks sessionID as a reflection session the way Fire
// does after a successful launch. Test seam only.
func (t *Tracker) tagReflectionSession(sessionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.reflectionSessions[sessionID] = true
}
