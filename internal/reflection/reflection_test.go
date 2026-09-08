package reflection

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

type fakeRuntime struct {
	messages  []Message
	created   []string
	marked    []string
	sent      []string
	budgets   []int64
	cancelled []string
	deleted   []string
	markErr   error
}

func newTracker(t *testing.T) (*Tracker, *fakeRuntime) {
	t.Helper()
	f := &fakeRuntime{messages: []Message{{Role: "user", Text: "Prefer concise answers"}}}
	rt := Runtime{
		LoadTranscript: func(context.Context, string) ([]Message, error) { return f.messages, nil },
		CreateSession: func(_ context.Context, title string) (string, error) {
			f.created = append(f.created, title)
			return "review-1", nil
		},
		MarkReview: func(_ context.Context, id string) error {
			f.marked = append(f.marked, id)
			return f.markErr
		},
		SendPromptWithBudget: func(_ context.Context, id, prompt string, budget int64) (string, error) {
			if len(f.marked) == 0 || f.marked[len(f.marked)-1] != id {
				t.Error("review sent before restricted-session marking")
			}
			f.sent = append(f.sent, prompt)
			f.budgets = append(f.budgets, budget)
			return id, nil
		},
		CancelSession: func(_ context.Context, id string) error {
			f.cancelled = append(f.cancelled, id)
			return nil
		},
		CleanupSession: func(_ context.Context, id string) error {
			f.deleted = append(f.deleted, id)
			return nil
		},
	}
	tracker := New(rt, nil)
	tracker.idleDelay = 0
	return tracker, f
}

func TestMemoryReviewCadenceAndForegroundLearningReset(t *testing.T) {
	tracker, _ := newTracker(t)
	tracker.Hydrate("s", MemoryInterval-1)
	tracker.UserTurnAccepted("s")
	review, _ := tracker.RunDone("s", "done", "", false)
	if !review.Memory || review.Skills {
		t.Fatalf("cadence review = %+v", review)
	}
	if again, _ := tracker.RunDone("s", "duplicate done", "", false); again.Any() {
		t.Fatal("duplicate completion repeated a review")
	}
	for i := 0; i < MemoryInterval-1; i++ {
		tracker.UserTurnAccepted("s")
	}
	tracker.LearningToolExecuted("s", "call", "mcp_gotack-memory_memory")
	tracker.UserTurnAccepted("s")
	if review, _ := tracker.RunDone("s", "done", "", false); review.Any() {
		t.Fatal("explicit memory use did not reset cadence")
	}
	tracker.Forget("s")
	if !tracker.NeedsHydration("s") {
		t.Fatal("deleted session retained learning state")
	}
}

func TestSkillIterationsNeverScheduleBackgroundSkillMutation(t *testing.T) {
	tracker, _ := newTracker(t)
	tracker.Hydrate("s", 0)
	for i := 0; i < 100; i++ {
		tracker.AssistantIteration("s", strings.Repeat("x", i+1), true)
	}
	if review, _ := tracker.RunDone("s", "done", "", false); review.Any() || review.Skills {
		t.Fatal("ordinary tool work scheduled automatic skill learning")
	}
	if (Review{Skills: true}).Any() {
		t.Fatal("skills-only background review is active")
	}
}

func TestHydrationDoesNotOverwriteLiveCounter(t *testing.T) {
	tracker, _ := newTracker(t)
	tracker.Hydrate("s", MemoryInterval-2)
	tracker.UserTurnAccepted("s")
	tracker.Hydrate("s", 0)
	tracker.UserTurnAccepted("s")
	if review, _ := tracker.RunDone("s", "done", "", false); !review.Memory {
		t.Fatal("repeated hydration clobbered live cadence")
	}
}

func TestCancelledFailedAndEmptyTurnsDoNotReview(t *testing.T) {
	for _, mode := range []string{"cancelled", "failed", "empty"} {
		t.Run(mode, func(t *testing.T) {
			tracker, _ := newTracker(t)
			tracker.Hydrate("s", MemoryInterval-1)
			tracker.UserTurnAccepted("s")
			text, runErr := "done", ""
			if mode == "failed" { runErr = "failed" }
			if mode == "empty" { text = "" }
			if review, _ := tracker.RunDone("s", text, runErr, mode == "cancelled"); review.Any() {
				t.Fatal("unsuccessful turn scheduled learning")
			}
		})
	}
}

func TestFireUsesRestrictedSessionAndCumulativeInputBudget(t *testing.T) {
	tracker, f := newTracker(t)
	if err := tracker.Fire(context.Background(), "source", Review{Memory: true}); err != nil {
		t.Fatal(err)
	}
	if len(f.created) != 1 || len(f.marked) != 1 || len(f.sent) != 1 || f.budgets[0] != MaxReviewInputTokens {
		t.Fatal("review launch or budget contract was not preserved")
	}
	if !strings.Contains(f.sent[0], "Prefer concise answers") || strings.Contains(f.sent[0], "Be ACTIVE") {
		t.Fatal("review lost personal evidence or retained automatic skill directives")
	}
	if err := tracker.Fire(context.Background(), "source", Review{Memory: true}); err == nil {
		t.Fatal("concurrent review launch was accepted")
	}
	if err := tracker.CancelForLiveTurn(context.Background(), "live"); err != nil || len(f.cancelled) != 1 {
		t.Fatal("foreground turn did not preempt review")
	}
	if review, id := tracker.RunDone("review-1", "Nothing to save.", "", false); review.Any() || id != "review-1" {
		t.Fatal("review completion failed to release its detached-session identity")
	}
}

func TestReviewCannotFallBackToAnUnbudgetedSend(t *testing.T) {
	tracker, _ := newTracker(t)
	tracker.rt.SendPromptWithBudget = nil
	called := false
	tracker.rt.SendPrompt = func(context.Context, string, string) (string, error) {
		called = true
		return "unexpected", nil
	}
	if err := tracker.Fire(context.Background(), "source", Review{Memory: true}); err == nil || called {
		t.Fatal("missing budget-aware runtime enabled unbounded review")
	}
}

func TestPartialReviewLaunchFailureCleansDetachedSession(t *testing.T) {
	tracker, f := newTracker(t)
	f.markErr = errors.New("marker unavailable")
	if err := tracker.Fire(context.Background(), "source", Review{Memory: true}); err == nil {
		t.Fatal("unmarked review launch succeeded")
	}
	if len(f.sent) != 0 || len(f.deleted) != 1 || f.deleted[0] != "review-1" || tracker.inflight {
		t.Fatal("partial launch leaked an active/unrestricted session")
	}
}

func TestIdleReviewIsCancelledBeforeAnyModelCall(t *testing.T) {
	tracker, f := newTracker(t)
	tracker.idleDelay = time.Hour
	finished := make(chan error, 1)
	go func() { finished <- tracker.Fire(context.Background(), "source", Review{Memory: true}) }()
	deadline := time.Now().Add(time.Second)
	for {
		tracker.mu.Lock()
		inflight := tracker.inflight
		tracker.mu.Unlock()
		if inflight { break }
		if time.Now().After(deadline) { t.Fatal("review launcher did not reserve its slot") }
		time.Sleep(time.Millisecond)
	}
	if err := tracker.CancelForLiveTurn(context.Background(), "live"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) { t.Fatalf("cancel error = %v", err) }
	case <-time.After(time.Second):
		t.Fatal("idle review did not stop")
	}
	if len(f.created) != 0 || len(f.sent) != 0 {
		t.Fatal("cancelled idle review reached the model")
	}
}

func TestFireLoadTimeoutReleasesReservation(t *testing.T) {
	tracker, _ := newTracker(t)
	tracker.fireTimeout = 20 * time.Millisecond
	tracker.rt.LoadTranscript = func(ctx context.Context, _ string) ([]Message, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if err := tracker.Fire(context.Background(), "source", Review{Memory: true}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout = %v", err)
	}
	if tracker.inflight {
		t.Fatal("timed-out launch retained its reservation")
	}
}

func TestReviewIterationsAreBoundedAndDeduplicated(t *testing.T) {
	tracker, _ := newTracker(t)
	if err := tracker.Fire(context.Background(), "source", Review{Memory: true}); err != nil { t.Fatal(err) }
	for i := 1; i < MaxReviewIterations; i++ {
		id := strings.Repeat("m", i)
		if tracker.AssistantIteration("review-1", id, true) || tracker.AssistantIteration("review-1", id, true) {
			t.Fatal("duplicate iteration exhausted the limit early")
		}
	}
	if !tracker.AssistantIteration("review-1", "limit", true) {
		t.Fatal("review exceeded its iteration limit")
	}
	if id, err := tracker.Stop(context.Background()); err != nil || id != "review-1" {
		t.Fatalf("stop = %q %v", id, err)
	}
}

func TestDigestIsBoundedAndOmitsToolDumps(t *testing.T) {
	messages := make([]Message, 50)
	for i := range messages {
		messages[i] = Message{Role: "user", Text: strings.Repeat("ệ ", 3000),
			Results: []ToolResult{{Name: "tool", Content: "tool-only-secret"}}}
	}
	messages[len(messages)-1] = Message{Role: "user", Text: "Newest important preference"}
	got := Digest(messages)
	if utf8.RuneCountInString(got) > maxDigestRunes || !utf8.ValidString(got) {
		t.Fatal("digest exceeded its budget or split Unicode")
	}
	if strings.Contains(got, "tool-only-secret") || !strings.Contains(got, "Newest important preference") {
		t.Fatal("digest exposed tools or lost the newest personal evidence")
	}
	prompt := Prompt(messages, Review{Memory: true})
	if !strings.Contains(prompt, "read-only evidence") || !strings.Contains(prompt, "Use only memory") {
		t.Fatal("review prompt lost its restricted-purpose boundary")
	}
}
