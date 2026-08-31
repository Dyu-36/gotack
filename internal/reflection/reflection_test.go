package reflection

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// fakeRuntime records every Runtime seam call so the firing sequence and its
// guards can be asserted end to end, the way internal/schedule tests do it.
type fakeRuntime struct {
	nextSessionID string
	createErr     error
	markErr       error
	sendErr       error
	preflightErr  error

	createdTitles  []string
	createdIDs     []string
	marked         []string
	sent           map[string]string // sessionID -> prompt
	createCalls    int
	markCalls      int
	sendCalls      int
	preflightCalls int
}

func (f *fakeRuntime) CreateSession(_ context.Context, title string) (string, error) {
	f.createCalls++
	if f.createErr != nil {
		return "", f.createErr
	}
	id := f.nextSessionID
	if id == "" {
		id = "reflection-session"
	}
	f.createdTitles = append(f.createdTitles, title)
	f.createdIDs = append(f.createdIDs, id)
	return id, nil
}

func (f *fakeRuntime) MarkUnattended(_ context.Context, sessionID string) error {
	f.markCalls++
	if f.markErr != nil {
		return f.markErr
	}
	f.marked = append(f.marked, sessionID)
	return nil
}

func (f *fakeRuntime) SendPrompt(_ context.Context, sessionID, prompt string) (string, error) {
	f.sendCalls++
	if f.sendErr != nil {
		return "", f.sendErr
	}
	if f.sent == nil {
		f.sent = map[string]string{}
	}
	f.sent[sessionID] = prompt
	return "run-1", nil
}

func (f *fakeRuntime) Preflight(context.Context) error {
	f.preflightCalls++
	return f.preflightErr
}

func newTestTracker(rt *fakeRuntime) *Tracker {
	tr := New(Runtime{
		CreateSession:  rt.CreateSession,
		MarkUnattended: rt.MarkUnattended,
		SendPrompt:     rt.SendPrompt,
		Preflight:      rt.Preflight,
	}, nil)
	tr.now = func() time.Time { return time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC) }
	return tr
}

func TestSessionDoneTurnGate(t *testing.T) {
	tests := []struct {
		name      string
		completed int
		wantFire  bool
	}{
		{name: "one turn below threshold does not trigger", completed: 1, wantFire: false},
		{name: "one turn short of threshold does not trigger", completed: DefaultTurnThreshold - 1, wantFire: false},
		{name: "threshold turn triggers", completed: DefaultTurnThreshold, wantFire: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tr := newTestTracker(&fakeRuntime{})
			got := false
			for i := 0; i < test.completed; i++ {
				got = tr.SessionDone("sess-1", "", false)
			}
			if got != test.wantFire {
				t.Fatalf("SessionDone after %d turns = %v, want %v", test.completed, got, test.wantFire)
			}
		})
	}
}

func TestSessionDoneIgnoresFailedAndCancelledRuns(t *testing.T) {
	tr := newTestTracker(&fakeRuntime{})
	for i := 0; i < DefaultTurnThreshold*2; i++ {
		if tr.SessionDone("sess-1", "provider error", false) {
			t.Fatal("errored run must never count toward the turn gate")
		}
		if tr.SessionDone("sess-1", "", true) {
			t.Fatal("cancelled run must never count toward the turn gate")
		}
	}
	if tr.SessionDone("sess-1", "", false) {
		t.Fatal("one successful turn is below the threshold and must not trigger")
	}
	if got := tr.turnCount("sess-1"); got != 1 {
		t.Fatalf("turn count = %d, want 1", got)
	}
}

func TestSessionEndGate(t *testing.T) {
	tests := []struct {
		name     string
		prepare  func(tr *Tracker)
		session  string
		wantFire bool
	}{
		{
			name:     "session with no turns does not trigger",
			prepare:  func(*Tracker) {},
			wantFire: false,
		},
		{
			name:     "session with completed turns triggers",
			prepare:  func(tr *Tracker) { tr.SessionDone("sess-1", "", false) },
			wantFire: true,
		},
		{
			name: "session end fires only once",
			prepare: func(tr *Tracker) {
				tr.SessionDone("sess-1", "", false)
				if !tr.SessionEnded("sess-1") {
					t.Fatal("first SessionEnded must trigger")
				}
			},
			wantFire: false,
		},
		{
			name:     "reflection session never triggers on end",
			prepare:  func(tr *Tracker) { tr.tagReflectionSession("reflection-session") },
			session:  "reflection-session",
			wantFire: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tr := newTestTracker(&fakeRuntime{})
			test.prepare(tr)
			session := test.session
			if session == "" {
				session = "sess-1"
			}
			if got := tr.SessionEnded(session); got != test.wantFire {
				t.Fatalf("SessionEnded(%q) = %v, want %v", session, got, test.wantFire)
			}
		})
	}
}

func TestFireSequenceIsCreateMarkSend(t *testing.T) {
	rt := &fakeRuntime{}
	tr := newTestTracker(rt)
	tr.turnThreshold = 1
	if !tr.SessionDone("sess-source", "", false) {
		t.Fatal("gate must open after one turn with threshold 1")
	}

	if err := tr.Fire(context.Background(), "sess-source"); err != nil {
		t.Fatalf("Fire() error = %v", err)
	}
	if rt.createCalls != 1 || rt.markCalls != 1 || rt.sendCalls != 1 {
		t.Fatalf("call counts = create %d mark %d send %d, want 1/1/1", rt.createCalls, rt.markCalls, rt.sendCalls)
	}
	if !strings.HasPrefix(rt.createdTitles[0], TitlePrefix) {
		t.Fatalf("reflection session title = %q, want %q prefix", rt.createdTitles[0], TitlePrefix)
	}
	if len(rt.marked) != 1 || rt.marked[0] != rt.createdIDs[0] {
		t.Fatalf("marked sessions = %v, want exactly the created session %v", rt.marked, rt.createdIDs)
	}
	if _, ok := rt.sent[rt.createdIDs[0]]; !ok {
		t.Fatalf("prompt never sent to the created session %q", rt.createdIDs[0])
	}
}

func TestFailedMarkAbortsSend(t *testing.T) {
	rt := &fakeRuntime{markErr: errors.New("roster unavailable")}
	tr := newTestTracker(rt)

	err := tr.Fire(context.Background(), "sess-source")
	if err == nil {
		t.Fatal("Fire() must fail when the unattended mark fails")
	}
	if rt.sendCalls != 0 {
		t.Fatalf("send ran %d times after a failed mark; the firing must abort before the prompt", rt.sendCalls)
	}
}

func TestLaunchFailureReleasesClaim(t *testing.T) {
	rt := &fakeRuntime{createErr: errors.New("engine down")}
	tr := newTestTracker(rt)

	if err := tr.Fire(context.Background(), "sess-source"); err == nil {
		t.Fatal("Fire() must surface a failed launch")
	}
	// The failed launch must not consume budget or hold the in-flight claim.
	rt.createErr = nil
	if err := tr.Fire(context.Background(), "sess-source"); err != nil {
		t.Fatalf("Fire() after a rolled-back failure error = %v, want success", err)
	}
}

func TestRecursionGuardIgnoresReflectionCompletions(t *testing.T) {
	rt := &fakeRuntime{}
	tr := newTestTracker(rt)
	tr.hourlyBudget = 2 // keep the budget out of this recursion-guard proof
	if err := tr.Fire(context.Background(), "sess-source"); err != nil {
		t.Fatalf("Fire() error = %v", err)
	}
	reflectionID := rt.createdIDs[0]

	for i := 0; i < DefaultTurnThreshold*2; i++ {
		if tr.SessionDone(reflectionID, "", false) {
			t.Fatal("completion of a tagged reflection session must never trigger another reflection")
		}
	}
	if got := tr.turnCount(reflectionID); got != 0 {
		t.Fatalf("reflection session counted %d turns, want 0", got)
	}
	// The reflection run's completion also releases the in-flight claim, so a
	// later gate decision can fire again.
	if err := tr.Fire(context.Background(), "sess-other"); err != nil {
		t.Fatalf("Fire() after the reflection completion error = %v, want success", err)
	}
}

func TestInflightBlocksConcurrentReflection(t *testing.T) {
	rt := &fakeRuntime{}
	tr := newTestTracker(rt)
	if err := tr.Fire(context.Background(), "sess-1"); err != nil {
		t.Fatalf("first Fire() error = %v", err)
	}
	err := tr.Fire(context.Background(), "sess-2")
	if err == nil {
		t.Fatal("second Fire() while a reflection run is in flight must be refused")
	}
	if rt.createCalls != 1 {
		t.Fatalf("create called %d times, want exactly 1", rt.createCalls)
	}
}

func TestHourlyBudgetCapsReflections(t *testing.T) {
	rt := &fakeRuntime{}
	tr := newTestTracker(rt)
	base := tr.now()

	if err := tr.Fire(context.Background(), "sess-1"); err != nil {
		t.Fatalf("first Fire() error = %v", err)
	}
	// The first reflection run completes: its own run_complete releases the
	// in-flight claim through the recursion guard.
	tr.SessionDone(rt.createdIDs[0], "", false)
	if err := tr.Fire(context.Background(), "sess-2"); err == nil {
		t.Fatal("second Fire() inside the budget window must be refused")
	}

	tr.now = func() time.Time { return base.Add(time.Hour + time.Minute) }
	if err := tr.Fire(context.Background(), "sess-3"); err != nil {
		t.Fatalf("Fire() after the window slides error = %v, want success", err)
	}
	if len(rt.createdIDs) != 2 {
		t.Fatalf("reflection runs launched = %d, want 2", len(rt.createdIDs))
	}
}

func TestPreflightSkipConsumesNothing(t *testing.T) {
	rt := &fakeRuntime{preflightErr: errors.New("no model configured")}
	tr := newTestTracker(rt)

	err := tr.Fire(context.Background(), "sess-source")
	if err == nil {
		t.Fatal("Fire() must surface a preflight skip")
	}
	if rt.createCalls != 0 {
		t.Fatalf("preflight skip still created %d sessions", rt.createCalls)
	}
	rt.preflightErr = nil
	if err := tr.Fire(context.Background(), "sess-source"); err != nil {
		t.Fatalf("Fire() after a preflight skip error = %v, want success", err)
	}
}

func TestFireResetsTurnCounterForTheSourceSession(t *testing.T) {
	rt := &fakeRuntime{}
	tr := newTestTracker(rt)
	tr.turnThreshold = 2
	tr.SessionDone("sess-1", "", false)
	tr.SessionDone("sess-1", "", false)
	if err := tr.Fire(context.Background(), "sess-1"); err != nil {
		t.Fatalf("Fire() error = %v", err)
	}
	if got := tr.turnCount("sess-1"); got != 0 {
		t.Fatalf("turn count after reflection = %d, want 0", got)
	}
	// One fresh turn is below the threshold again.
	if tr.SessionDone("sess-1", "", false) {
		t.Fatal("gate must reopen only after a full fresh threshold")
	}
}

func TestFireRequiresSourceSession(t *testing.T) {
	tr := newTestTracker(&fakeRuntime{})
	if err := tr.Fire(context.Background(), ""); err == nil {
		t.Fatal("Fire() with an empty source session must fail")
	}
}

// TestPromptRoutesMemoryThroughD3 pins the D3 routing of the reflection
// prompt: memory entries may only be proposed through the gotack-memory tool,
// every other write path is named and forbidden.
func TestPromptRoutesMemoryThroughD3(t *testing.T) {
	prompt := PromptFor("sess-source")
	for _, required := range []string{
		"sess-source",
		"memory tool",
		"gotack-memory",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("reflection prompt missing %q; got:\n%s", required, prompt)
		}
	}
	for _, forbidden := range []string{
		"write context files",
	} {
		if !strings.Contains(prompt, forbidden) {
			t.Fatalf("reflection prompt must forbid direct context writes (%q); got:\n%s", forbidden, prompt)
		}
	}
}

func TestSessionDoneEmptySessionIDIsIgnored(t *testing.T) {
	tr := newTestTracker(&fakeRuntime{})
	if tr.SessionDone("", "", false) {
		t.Fatal("empty session id must never trigger")
	}
	if tr.SessionEnded("") {
		t.Fatal("empty session id must never trigger on session end")
	}
}
