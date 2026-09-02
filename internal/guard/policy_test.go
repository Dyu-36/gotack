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
	contextDir := filepath.Join(root, "context")
	outside := filepath.Join(t.TempDir(), "elsewhere.txt")

	cases := []struct {
		name      string
		in        Input
		opts      Options
		want      string
		reasonHas string
		wantHalt  bool
	}{
		{
			name: "blocklist beats everything even interactively",
			in:   input(t, root, "bash", map[string]any{"command": "format C:"}),
			opts: Options{WriteSafeRoot: root},
			want: DecisionDeny, reasonHas: ruleDiskFormatWipe, wantHalt: true,
		},
		{
			name: "read tool is auto-approved interactively",
			in:   input(t, root, "view", map[string]any{"file_path": outside}),
			opts: Options{WriteSafeRoot: root},
			want: DecisionAllow,
		},
		{
			name: "read tool is auto-approved unattended",
			in:   input(t, root, "grep", map[string]any{"pattern": "x"}),
			opts: Options{WriteSafeRoot: root, Unattended: true},
			want: DecisionAllow,
		},
		{
			name: "write inside the safe root is auto-approved",
			in:   input(t, root, "write", map[string]any{"file_path": filepath.Join(root, "notes.txt")}),
			opts: Options{WriteSafeRoot: root},
			want: DecisionAllow,
		},
		{
			name: "relative write resolving inside the root is auto-approved",
			in:   input(t, root, "edit", map[string]any{"file_path": "src/main.go"}),
			opts: Options{WriteSafeRoot: root},
			want: DecisionAllow,
		},
		{
			name: "write outside the safe root is denied",
			in:   input(t, root, "write", map[string]any{"file_path": outside}),
			opts: Options{WriteSafeRoot: root},
			want: DecisionDeny, reasonHas: ruleWriteOutsideRoot,
		},
		{
			name: "write into the memory context dir is denied even inside the root",
			in:   input(t, root, "write", map[string]any{"file_path": filepath.Join(contextDir, "memory.md")}),
			opts: Options{WriteSafeRoot: root, ContextDir: contextDir},
			want: DecisionDeny, reasonHas: ruleContextWrite,
		},
		{
			name: "context-dir denial names the memory rule before the root rule",
			in:   input(t, root, "multiedit", map[string]any{"file_path": filepath.Join(contextDir, "caps.md")}),
			opts: Options{WriteSafeRoot: "", ContextDir: contextDir},
			want: DecisionDeny, reasonHas: ruleContextWrite,
		},
		{
			name: "write boundary beats unattended posture",
			in:   input(t, root, "write", map[string]any{"file_path": outside}),
			opts: Options{WriteSafeRoot: root, Unattended: true},
			want: DecisionDeny, reasonHas: ruleWriteOutsideRoot,
		},
		{
			name: "write without a path falls through to ask interactively",
			in:   input(t, root, "write", map[string]any{}),
			opts: Options{WriteSafeRoot: root},
			want: DecisionNone,
		},
		{
			name: "write without a path is denied unattended",
			in:   input(t, root, "write", map[string]any{}),
			opts: Options{WriteSafeRoot: root, Unattended: true},
			want: DecisionDeny, reasonHas: ruleUnattendedApproval,
		},
		{
			name: "benign shell command asks interactively",
			in:   input(t, root, "bash", map[string]any{"command": "go build ./..."}),
			opts: Options{WriteSafeRoot: root},
			want: DecisionNone,
		},
		{
			name: "shell command is denied unattended instead of prompting",
			in:   input(t, root, "bash", map[string]any{"command": "go build ./..."}),
			opts: Options{WriteSafeRoot: root, Unattended: true},
			want: DecisionDeny, reasonHas: ruleUnattendedApproval,
		},
		{
			name: "network fetch asks interactively",
			in:   input(t, root, "download", map[string]any{"url": "https://example.com/file.zip"}),
			opts: Options{WriteSafeRoot: root},
			want: DecisionNone,
		},
		{
			name: "network fetch is denied unattended",
			in:   input(t, root, "fetch", map[string]any{"url": "https://example.com"}),
			opts: Options{WriteSafeRoot: root, Unattended: true},
			want: DecisionDeny, reasonHas: ruleUnattendedApproval,
		},
		{
			name: "delegation is denied unattended (sub-agent hole mitigation)",
			in:   input(t, root, "agent", map[string]any{"prompt": "do it"}),
			opts: Options{WriteSafeRoot: root, Unattended: true},
			want: DecisionDeny, reasonHas: ruleUnattendedApproval,
		},
		{
			name: "unknown tool asks interactively",
			in:   input(t, root, "future_tool", map[string]any{}),
			opts: Options{WriteSafeRoot: root},
			want: DecisionNone,
		},
		{
			name: "empty safe root disables the root check",
			in:   input(t, root, "write", map[string]any{"file_path": outside}),
			opts: Options{},
			want: DecisionAllow,
		},
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
		{"sibling prefix dir is outside", root, root + "-x", false},
		{"nested sibling prefix is outside", root, filepath.Join(root+"-x", "b.txt"), false},
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
	allowed := []string{
		"ls",
		"glob",
		"grep",
		"view",
		"mcp_gotack-memory_memory",
		"mcp_gotack-skills_skill_view",
		"mcp_gotack-skills_skill_manage",
	}
	for _, tool := range allowed {
		t.Run("allows "+tool, func(t *testing.T) {
			got := Evaluate(input(t, root, tool, map[string]any{}), opts)
			if got.Decision != DecisionAllow {
				t.Fatalf("decision = %q (reason %q), want allow", got.Decision, got.Reason)
			}
		})
	}

	denied := []string{
		"sourcegraph",
		"bash",
		"write",
		"download",
		"fetch",
		"agent",
		"future_tool",
		"mcp_untrusted_memory",
		"mcp_untrusted_skill_manage",
	}
	for _, tool := range denied {
		t.Run("denies "+tool, func(t *testing.T) {
			toolInput := map[string]any{}
			if tool == "write" {
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
	got := Evaluate(
		input(t, t.TempDir(), "bash", map[string]any{"command": "format C:"}),
		Options{Unattended: true, Review: true},
	)
	if got.Decision != DecisionDeny || !got.Halt || !strings.Contains(got.Reason, ruleDiskFormatWipe) {
		t.Fatalf("got %+v, want unrecoverable-command denial before review policy", got)
	}
}

func TestSkillContextInjection(t *testing.T) {
	tools := []string{
		"mcp_gotack-skills_skill_view",
		"mcp_gotack-skills_skill_manage",
	}
	for _, tool := range tools {
		t.Run("foreground "+tool, func(t *testing.T) {
			in := input(t, t.TempDir(), tool, map[string]any{
				"_session_id":        "forged",
				"_background_review": true,
			})
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
			if got.Decision != DecisionAllow {
				t.Fatalf("review decision = %q, want allow", got.Decision)
			}
			assertSkillPatch(t, got, "review-session", true)
		})
	}

	memory := input(t, t.TempDir(), "mcp_gotack-memory_memory", map[string]any{})
	memory.SessionID = "review-session"
	if got := Evaluate(memory, Options{Review: true}); len(got.UpdatedInput) != 0 {
		t.Fatalf("memory must not receive skills-only hidden fields: %s", got.UpdatedInput)
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
