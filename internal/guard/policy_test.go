package guard

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// policy_test.go -- role: table tests for the graduated tier decisions
// (ADR 0002, plan Phase 4): the blocklist floor, the write-safe root, the
// memory context dir, and the unattended-deny posture.

// input builds a hook payload for one tool call rooted at cwd.
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
