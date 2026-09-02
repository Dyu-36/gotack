package reflection

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

type fakeRuntime struct {
	mu        sync.Mutex
	calls     []string
	messages  []Message
	markErr   error
	budget    int64
	useBudget bool
}

func (f *fakeRuntime) runtime() Runtime {
	rt := Runtime{
		LoadTranscript: func(_ context.Context, source string) ([]Message, error) {
			f.record("load:" + source)
			return f.messages, nil
		},
		CreateSession: func(context.Context, string) (string, error) {
			f.record("create")
			return "review-1", nil
		},
		MarkReview: func(context.Context, string) error {
			f.record("mark")
			return f.markErr
		},
		SendPrompt: func(_ context.Context, id, prompt string) (string, error) {
			f.record("send:" + id)
			if !strings.Contains(prompt, "Review the conversation") {
				return "", errors.New("missing instructions")
			}
			return "run-1", nil
		},
		CancelSession: func(_ context.Context, id string) error {
			f.record("cancel:" + id)
			return nil
		},
		CleanupSession: func(_ context.Context, id string) error {
			f.record("cleanup:" + id)
			return nil
		},
	}
	if f.useBudget {
		rt.SendPromptWithBudget = func(_ context.Context, id, prompt string, budget int64) (string, error) {
			f.record("send-budget:" + id)
			f.budget = budget
			if !strings.Contains(prompt, "Review the conversation") {
				return "", errors.New("missing instructions")
			}
			return "run-1", nil
		}
	}
	return rt
}

func TestFireUsesHermesReviewInputBudget(t *testing.T) {
	fake := &fakeRuntime{useBudget: true}
	if err := newTracker(fake.runtime()).Fire(t.Context(), "source", Review{Memory: true}); err != nil {
		t.Fatal(err)
	}
	if fake.budget != MaxReviewInputTokens || !strings.Contains(fmt.Sprint(fake.snapshot()), "send-budget:review-1") {
		t.Fatalf("review budget = %d, calls = %v", fake.budget, fake.snapshot())
	}
}

func (f *fakeRuntime) record(call string) {
	f.mu.Lock()
	f.calls = append(f.calls, call)
	f.mu.Unlock()
}

func (f *fakeRuntime) snapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func newTracker(rt Runtime) *Tracker { return New(rt, slog.New(slog.DiscardHandler)) }

func TestHermesCadenceAndFailureGate(t *testing.T) {
	tracker := newTracker(Runtime{})
	tracker.Hydrate("s", 9)
	tracker.UserTurnAccepted("s")
	for i := 0; i < SkillInterval; i++ {
		tracker.AssistantIteration("s", fmt.Sprintf("m-%d", i), true)
	}
	if review, _ := tracker.RunDone("s", "partial", "", true); review.Any() {
		t.Fatalf("cancelled run reviewed: %+v", review)
	}
	if review, _ := tracker.RunDone("s", "next", "", false); review.Any() {
		t.Fatalf("consumed cadence leaked: %+v", review)
	}
}

func TestLateSkillManageSnapshotResetsWithoutDoubleCount(t *testing.T) {
	tracker := newTracker(Runtime{})
	tracker.Hydrate("s", 0)
	for i := 0; i < 9; i++ {
		tracker.AssistantIteration("s", fmt.Sprintf("before-%d", i), true)
	}
	tracker.AssistantIteration("s", "manage", false)
	tracker.AssistantIteration("s", "manage", true)
	tracker.LearningToolExecuted("s", "call-1", "skill_manage")
	for i := 0; i < 9; i++ {
		tracker.AssistantIteration("s", fmt.Sprintf("after-%d", i), true)
	}
	if review, _ := tracker.RunDone("s", "ok", "", false); review.Any() {
		t.Fatalf("skill cadence was not reset: %+v", review)
	}
}

func TestSkillCadenceDoesNotDependOnMemoryHydration(t *testing.T) {
	tracker := newTracker(Runtime{})
	for i := 0; i < SkillInterval; i++ {
		tracker.AssistantIteration("s", fmt.Sprintf("m-%d", i), true)
	}
	review, _ := tracker.RunDone("s", "ok", "", false)
	if !review.Skills || review.Memory {
		t.Fatalf("review = %+v", review)
	}
}

func TestLearningToolResetsOnlyAdmittedCadenceAndDeduplicates(t *testing.T) {
	tracker := newTracker(Runtime{})
	tracker.Hydrate("s", 0)
	tracker.UserTurnAccepted("s")
	tracker.AssistantIteration("s", "m-1", true)
	tracker.LearningToolExecuted("s", "memory-1", "memory")
	tracker.LearningToolExecuted("s", "memory-1", "memory")
	for i := 1; i < MemoryInterval; i++ {
		tracker.UserTurnAccepted("s")
	}
	if review, _ := tracker.RunDone("s", "ok", "", false); review.Memory {
		t.Fatalf("duplicate memory result reset cadence incorrectly: %+v", review)
	}

	for i := 0; i < SkillInterval; i++ {
		tracker.AssistantIteration("s", fmt.Sprintf("skill-%d", i), true)
	}
	tracker.LearningToolExecuted("s", "skill-1", "skill_manage")
	tracker.LearningToolExecuted("s", "skill-1", "skill_manage")
	for i := 0; i < SkillInterval-1; i++ {
		tracker.AssistantIteration("s", fmt.Sprintf("after-%d", i), true)
	}
	if review, _ := tracker.RunDone("s", "ok", "", false); review.Skills {
		t.Fatalf("duplicate skill result reset cadence incorrectly: %+v", review)
	}
}

func TestMemoryResultDoesNotClearAlreadyDueReview(t *testing.T) {
	tracker := newTracker(Runtime{})
	tracker.Hydrate("s", 9)
	tracker.UserTurnAccepted("s")
	tracker.LearningToolExecuted("s", "memory-1", "memory")
	if review, _ := tracker.RunDone("s", "ok", "", false); !review.Memory {
		t.Fatal("admitted memory call cleared the review due for this turn")
	}
}

func TestFireOrderFailureCleanupAndSingleInflight(t *testing.T) {
	fake := &fakeRuntime{messages: []Message{{Role: "user", Text: "preference"}}}
	tracker := newTracker(fake.runtime())
	if err := tracker.Fire(t.Context(), "source", Review{Memory: true}); err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(fake.snapshot()); got != "[load:source create mark send:review-1]" {
		t.Fatalf("launch order = %s", got)
	}
	if err := tracker.Fire(t.Context(), "other", Review{Skills: true}); err == nil {
		t.Fatal("second review launched")
	}
	if _, cleanup := tracker.RunDone("review-1", "done", "", false); cleanup != "review-1" {
		t.Fatalf("cleanup id = %q", cleanup)
	}

	failed := &fakeRuntime{markErr: errors.New("roster")}
	if err := newTracker(failed.runtime()).Fire(t.Context(), "source", Review{Skills: true}); err == nil {
		t.Fatal("mark failure was ignored")
	}
	if !strings.Contains(fmt.Sprint(failed.snapshot()), "cleanup:review-1") {
		t.Fatalf("failed launch not cleaned: %v", failed.snapshot())
	}
}

func TestReviewCeilingAcceptsLateToolMetadata(t *testing.T) {
	fake := &fakeRuntime{}
	tracker := newTracker(fake.runtime())
	if err := tracker.Fire(t.Context(), "source", Review{Skills: true}); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= MaxReviewIterations; i++ {
		if tracker.AssistantIteration("review-1", fmt.Sprintf("m-%d", i), false) {
			t.Fatalf("text-only iteration %d cancelled", i)
		}
	}
	if !tracker.AssistantIteration("review-1", "m-16", true) {
		t.Fatal("late tool metadata did not enforce ceiling")
	}
}

func TestStopCancelsAndReturnsDetachedSession(t *testing.T) {
	fake := &fakeRuntime{}
	tracker := newTracker(fake.runtime())
	if err := tracker.Fire(t.Context(), "source", Review{Memory: true}); err != nil {
		t.Fatal(err)
	}
	id, err := tracker.Stop(t.Context())
	if err != nil || id != "review-1" || !strings.Contains(fmt.Sprint(fake.snapshot()), "cancel:review-1") {
		t.Fatalf("Stop = id %q err %v calls %v", id, err, fake.snapshot())
	}
	if err := tracker.Fire(t.Context(), "other", Review{Skills: true}); err != nil {
		t.Fatalf("stop left review claim: %v", err)
	}
}

func TestDigestBoundsOldTextAndKeepsRecentToolPair(t *testing.T) {
	messages := make([]Message, 26)
	messages[0] = Message{Role: "user", Text: strings.Repeat("u", 301)}
	messages[1] = Message{Role: "assistant", Tools: []string{"read"}}
	messages[2] = Message{Role: "tool", Results: []ToolResult{{Name: "read", Content: "file body"}}}
	for i := 3; i < len(messages); i++ {
		messages[i] = Message{Role: "user", Text: fmt.Sprintf("recent-%d", i)}
	}
	digest := Digest(messages)
	if strings.Contains(digest, strings.Repeat("u", 301)) ||
		!strings.Contains(digest, strings.Repeat("u", 300)) ||
		!strings.Contains(digest, "ASSISTANT[tools: read]") ||
		!strings.Contains(digest, "TOOL[read]:\nfile body") {
		t.Fatalf("digest drifted: %s", digest)
	}
}

func TestDigestAppliesHermesToolResultBudgets(t *testing.T) {
	large := strings.Repeat("x", maxToolResultRunes+1)
	mcp := strings.Repeat("m", maxMCPResultRunes+1)
	messages := []Message{
		{Role: "assistant", Tools: []string{"read"}},
		{Role: "tool", Results: []ToolResult{{Name: "read", Content: large}}},
		{Role: "tool", Results: []ToolResult{{Name: "mcp_gotack-recall_session_search", Content: mcp}}},
	}
	digest := Digest(messages)
	if strings.Contains(digest, large) || strings.Contains(digest, mcp) ||
		!strings.Contains(digest, "Tool result truncated") {
		t.Fatal("oversized tool result was not reduced to a preview")
	}

	tooMany := []Message{{Role: "assistant", Tools: []string{"a"}}}
	for i := 0; i < 3; i++ {
		tooMany = append(tooMany, Message{Role: "tool", Results: []ToolResult{{Name: "read", Content: strings.Repeat("z", 100_000)}}})
	}
	bounded := boundToolResults(tooMany)
	total := 0
	for _, message := range bounded {
		for _, result := range message.Results {
			total += runeLen(result.Content)
		}
	}
	if total > maxTurnToolResultRunes {
		t.Fatalf("tool turn budget = %d, want <= %d", total, maxTurnToolResultRunes)
	}
}

func TestCombinedPromptDoesNotStopBeforeSkillOrMemoryReview(t *testing.T) {
	prompt := Prompt([]Message{{Role: "user", Text: "Remember that I prefer CSV."}}, Review{Memory: true, Skills: true})
	if !strings.Contains(prompt, "Act on either dimension that has a real signal") ||
		strings.Contains(prompt, "If nothing is worth saving, just say") ||
		strings.Contains(prompt, "no correction or reusable technique") {
		t.Fatalf("combined prompt has an early single-dimension stop: %s", prompt)
	}
}
