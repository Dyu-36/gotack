package guard

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func input(t *testing.T, cwd, tool string, toolInput map[string]any) Input {
	t.Helper()
	raw, err := json.Marshal(toolInput)
	if err != nil {
		t.Fatalf("marshal tool input: %v", err)
	}
	return Input{Event: "PreToolUse", SessionID: "s", CWD: cwd, ToolName: tool, ToolInput: raw}
}

func TestEvaluateTierMatrix(t *testing.T) {
	root := t.TempDir()
	contextDir := filepath.Join(root, "assistant")
	outside := filepath.Join(t.TempDir(), "elsewhere.txt")
	cases := []struct {
		name      string
		in        Input
		opts      Options
		want      string
		reasonHas string
		wantHalt  bool
	}{
		{"blocklist beats interactive policy", input(t, root, "bash", map[string]any{"command": "format C:"}), Options{WriteSafeRoot: root}, DecisionDeny, ruleDiskFormatWipe, true},
		{"read outside root is approved", input(t, root, "view", map[string]any{"file_path": outside}), Options{WriteSafeRoot: root}, DecisionAllow, "", false},
		{"unattended read is approved", input(t, root, "grep", map[string]any{"pattern": "x"}), Options{WriteSafeRoot: root, Unattended: true}, DecisionAllow, "", false},
		{"write inside safe root", input(t, root, "write", map[string]any{"file_path": filepath.Join(root, "notes.txt")}), Options{WriteSafeRoot: root}, DecisionAllow, "", false},
		{"relative write inside root", input(t, root, "edit", map[string]any{"file_path": "src/main.go"}), Options{WriteSafeRoot: root}, DecisionAllow, "", false},
		{"write outside safe root", input(t, root, "write", map[string]any{"file_path": outside}), Options{WriteSafeRoot: root}, DecisionDeny, ruleWriteOutsideRoot, false},
		{"personal memory protected inside root", input(t, root, "write", map[string]any{"file_path": filepath.Join(contextDir, "MEMORY.md")}), Options{WriteSafeRoot: root, ContextDir: contextDir}, DecisionDeny, ruleContextWrite, false},
		{"personal memory rule before root rule", input(t, root, "multiedit", map[string]any{"file_path": filepath.Join(contextDir, "PROFILE.md")}), Options{ContextDir: contextDir}, DecisionDeny, ruleContextWrite, false},
		{"write boundary before unattended posture", input(t, root, "write", map[string]any{"file_path": outside}), Options{WriteSafeRoot: root, Unattended: true}, DecisionDeny, ruleWriteOutsideRoot, false},
		{"write without path asks interactively", input(t, root, "write", map[string]any{}), Options{WriteSafeRoot: root}, DecisionNone, "", false},
		{"write without path denied unattended", input(t, root, "write", map[string]any{}), Options{WriteSafeRoot: root, Unattended: true}, DecisionDeny, ruleUnattendedApproval, false},
		{"benign shell asks interactively", input(t, root, "bash", map[string]any{"command": "go build ./..."}), Options{WriteSafeRoot: root}, DecisionNone, "", false},
		{"shell denied unattended", input(t, root, "bash", map[string]any{"command": "go build ./..."}), Options{WriteSafeRoot: root, Unattended: true}, DecisionDeny, ruleUnattendedApproval, false},
		{"download asks interactively", input(t, root, "download", map[string]any{"url": "https://example.com/file.zip"}), Options{WriteSafeRoot: root}, DecisionNone, "", false},
		{"fetch denied unattended", input(t, root, "fetch", map[string]any{"url": "https://example.com"}), Options{WriteSafeRoot: root, Unattended: true}, DecisionDeny, ruleUnattendedApproval, false},
		{"delegation denied unattended", input(t, root, "agent", map[string]any{"prompt": "do it"}), Options{WriteSafeRoot: root, Unattended: true}, DecisionDeny, ruleUnattendedApproval, false},
		{"question uses plain text unattended", input(t, root, "question", map[string]any{"questions": []any{}}), Options{WriteSafeRoot: root, Unattended: true}, DecisionDeny, ruleUnattendedQuestion, false},
		{"unknown tool asks interactively", input(t, root, "future_tool", map[string]any{}), Options{WriteSafeRoot: root}, DecisionNone, "", false},
		{"empty safe root disables root check", input(t, root, "write", map[string]any{"file_path": outside}), Options{}, DecisionAllow, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Evaluate(tc.in, tc.opts)
			if got.Decision != tc.want {
				t.Fatalf("decision = %q (reason %q), want %q", got.Decision, got.Reason, tc.want)
			}
			if tc.want == DecisionDeny {
				if !strings.Contains(got.Reason, tc.reasonHas) {
					t.Fatalf("reason %q must name rule %q", got.Reason, tc.reasonHas)
				}
				if got.Halt != tc.wantHalt {
					t.Fatalf("halt = %v, want %v", got.Halt, tc.wantHalt)
				}
			}
		})
	}
}

func TestUnattendedQuestionReasonRequiresPlainTextFallback(t *testing.T) {
	got := Evaluate(input(t, t.TempDir(), "question", map[string]any{}), Options{Unattended: true})
	if got.Decision != DecisionDeny || !strings.Contains(got.Reason, "ask for the missing information directly") {
		t.Fatalf("got %+v, want plain-text fallback instruction", got)
	}
}

func TestWithinPathBoundaries(t *testing.T) {
	root := t.TempDir()
	cases := []struct {
		name   string
		root   string
		target string
		want   bool
	}{
		{"exact root", root, root, true},
		{"nested file", root, filepath.Join(root, "a", "b.txt"), true},
		{"sibling prefix outside", root, root + "-x", false},
		{"nested sibling prefix outside", root, filepath.Join(root+"-x", "b.txt"), false},
		{"empty root matches nothing", "", filepath.Join(root, "a"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := withinPath(tc.root, tc.target); got != tc.want {
				t.Fatalf("withinPath(%q, %q) = %v, want %v", tc.root, tc.target, got, tc.want)
			}
		})
	}
}

func TestBackgroundReviewWhitelist(t *testing.T) {
	root := t.TempDir()
	opts := Options{WriteSafeRoot: root, Unattended: true, Review: true}
	for _, tool := range []string{"memory", "mcp_gotack-memory_memory"} {
		t.Run("allows "+tool, func(t *testing.T) {
			got := Evaluate(input(t, root, tool, map[string]any{}), opts)
			if got.Decision != DecisionAllow {
				t.Fatalf("decision = %q (reason %q), want allow", got.Decision, got.Reason)
			}
		})
	}
	denied := []string{
		"ls", "glob", "grep", "view", "sourcegraph", "bash", "write", "edit", "multiedit",
		"download", "fetch", "agent", "question", "future_tool", "mcp_untrusted_memory",
		"skill_view", "skill_manage", "mcp_gotack-skills_skill_view", "mcp_gotack-skills_skill_manage",
		"mcp_untrusted_skill_manage",
	}
	for _, tool := range denied {
		t.Run("denies "+tool, func(t *testing.T) {
			toolInput := map[string]any{}
			if isWriteTool(tool) {
				toolInput["file_path"] = filepath.Join(root, "safe.txt")
			}
			got := Evaluate(input(t, root, tool, toolInput), opts)
			if got.Decision != DecisionDeny || !strings.Contains(got.Reason, ruleReviewWhitelist) {
				t.Fatalf("got %+v, want review-whitelist denial", got)
			}
		})
	}
}

func TestBackgroundReviewKeepsSecurityFloor(t *testing.T) {
	got := Evaluate(input(t, t.TempDir(), "bash", map[string]any{"command": "format C:"}), Options{Unattended: true, Review: true})
	if got.Decision != DecisionDeny || !got.Halt || !strings.Contains(got.Reason, ruleDiskFormatWipe) {
		t.Fatalf("got %+v, want destructive-command denial before review policy", got)
	}
}

func TestSkillContextInjection(t *testing.T) {
	for _, tool := range []string{"mcp_gotack-skills_skill_view", "mcp_gotack-skills_skill_manage"} {
		t.Run("foreground "+tool, func(t *testing.T) {
			in := input(t, t.TempDir(), tool, map[string]any{"_session_id": "forged", "_background_review": true})
			in.SessionID = "trusted-session"
			got := Evaluate(in, Options{})
			if got.Decision != DecisionNone {
				t.Fatalf("foreground decision = %q, want normal interactive pass-through", got.Decision)
			}
			assertSkillPatch(t, got, "trusted-session", false)
		})
		t.Run("review "+tool, func(t *testing.T) {
			in := input(t, t.TempDir(), tool, map[string]any{})
			in.SessionID = "review-session"
			got := Evaluate(in, Options{Unattended: true, Review: true})
			if got.Decision != DecisionDeny || !strings.Contains(got.Reason, ruleReviewWhitelist) || len(got.UpdatedInput) != 0 {
				t.Fatalf("got %+v, want memory-only review to reject skill operations", got)
			}
		})
	}
	memory := input(t, t.TempDir(), "mcp_gotack-memory_memory", map[string]any{})
	memory.SessionID = "review-session"
	if got := Evaluate(memory, Options{Review: true}); len(got.UpdatedInput) != 0 {
		t.Fatalf("memory must not receive skills-only fields: %s", got.UpdatedInput)
	}
}

func assertSkillPatch(t *testing.T, out Output, sessionID string, review bool) {
	t.Helper()
	if out.IsNone() {
		t.Fatal("skill context patch must be emitted even with no decision")
	}
	var patch map[string]any
	if err := json.Unmarshal(out.UpdatedInput, &patch); err != nil {
		t.Fatalf("decode updated_input: %v", err)
	}
	if len(patch) != 2 || patch["_session_id"] != sessionID || patch["_background_review"] != review {
		t.Fatalf("updated_input = %s, want session=%q review=%v", out.UpdatedInput, sessionID, review)
	}
}
