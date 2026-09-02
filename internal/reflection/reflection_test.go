package reflection

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
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

func TestDigestKeepsOnlyLatestTwentyFourItems(t *testing.T) {
	messages := make([]Message, digestTail+2)
	for index := range messages {
		messages[index] = Message{Role: "user", Text: fmt.Sprintf("message-%02d", index)}
	}

	digest := Digest(messages)
	if strings.Contains(digest, "message-00") || strings.Contains(digest, "message-01") {
		t.Fatalf("digest retained items older than the latest %d: %s", digestTail, digest)
	}
	if !strings.Contains(digest, "message-02") || !strings.Contains(digest, "message-25") {
		t.Fatalf("digest dropped a retained boundary item: %s", digest)
	}
	if got := strings.Count(digest, "[USER]\n"); got != digestTail {
		t.Fatalf("digest item count = %d, want %d", got, digestTail)
	}
}

func TestDigestBoundsMessageAndToolPreviewsByRunes(t *testing.T) {
	digest := Digest([]Message{
		{Role: "user", Text: strings.Repeat("界", messagePreviewRunes+25) + "USER-END"},
		{Role: "assistant", Text: strings.Repeat("答", messagePreviewRunes+25) + "ASSISTANT-END", Tools: []string{"read"}},
		{Role: "tool", Results: []ToolResult{{Name: "read", Content: strings.Repeat("工", toolResultPreviewRunes+25) + "TOOL-END"}}},
	})

	userPreview := digestLineAfter(t, digest, "[USER]")
	assistantPreview := digestLineAfter(t, digest, "[ASSISTANT]")
	toolPreview := strings.TrimPrefix(
		digestLineWithPrefix(t, digest, "TOOL[read]: "),
		"TOOL[read]: ",
	)
	for label, value := range map[string]string{
		"user": userPreview, "assistant": assistantPreview,
	} {
		if got := runeLen(value); got != messagePreviewRunes {
			t.Fatalf("%s preview runes = %d, want %d", label, got, messagePreviewRunes)
		}
	}
	if got := runeLen(toolPreview); got != toolResultPreviewRunes {
		t.Fatalf("tool preview runes = %d, want %d", got, toolResultPreviewRunes)
	}
	if strings.Contains(digest, "USER-END") || strings.Contains(digest, "ASSISTANT-END") || strings.Contains(digest, "TOOL-END") {
		t.Fatalf("digest leaked content beyond a preview boundary: %s", digest)
	}
	if strings.Count(digest, truncationMarker) != 3 || !utf8.ValidString(digest) {
		t.Fatalf("digest truncation was not explicit and rune-safe: %s", digest)
	}
}

func digestLineAfter(t *testing.T, digest, header string) string {
	t.Helper()
	lines := strings.Split(digest, "\n")
	for index, line := range lines {
		if line == header && index+1 < len(lines) {
			return lines[index+1]
		}
	}
	t.Fatalf("missing digest header %q in %s", header, digest)
	return ""
}

func digestLineWithPrefix(t *testing.T, digest, prefix string) string {
	t.Helper()
	for _, line := range strings.Split(digest, "\n") {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	t.Fatalf("missing digest line prefix %q in %s", prefix, digest)
	return ""
}

func TestCombinedPromptDoesNotStopBeforeSkillOrMemoryReview(t *testing.T) {
	prompt := Prompt([]Message{{Role: "user", Text: "Remember that I prefer CSV."}}, Review{Memory: true, Skills: true})
	if !strings.Contains(prompt, "Act on either dimension that has a real signal") ||
		strings.Contains(prompt, "If nothing is worth saving, just say") ||
		strings.Contains(prompt, "no correction or reusable technique") {
		t.Fatalf("combined prompt has an early single-dimension stop: %s", prompt)
	}
}
