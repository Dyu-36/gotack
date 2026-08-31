package guard

import (
	"fmt"
)

// policy.go -- role: evaluate one tool call and return the hook decision.
//
// The guard applies the graduated posture of ADR 0002 and plan Phase 4:
//
//   - deny — the unrecoverable-command blocklist is never overridable in any
//     posture; writes outside the write-safe root and writes into the memory
//     context directory are refused outright.
//   - auto — ordinary reads anywhere and edits inside the write-safe root are
//     pre-approved, so interactive work is not prompted for them.
//   - ask — everything else carries no opinion, so Crush's own permission
//     relay asks the user. An unattended session has nobody to answer, so the
//     same operations are denied with a legible reason instead: a prompt that
//     can never be answered must fail the run, not hang it (plan 5.4).

// Rule names for the write-boundary denials. They appear verbatim in the
// refusal reason, matching the blocklist convention (outcome 4).
const (
	ruleContextWrite       = "memory-context-write"
	ruleWriteOutsideRoot   = "write-outside-safe-root"
	ruleUnattendedApproval = "unattended-approval"
)

// Options parameterises one Evaluate call for the session being guarded.
type Options struct {
	// WriteSafeRoot is the only directory file-writing tools may touch;
	// writes outside it are denied. Empty disables the root check.
	WriteSafeRoot string
	// ContextDir is Gotack's memory context directory. Writes into it are
	// always denied regardless of the safe root, closing the cap-bypass hole
	// of decision 0003 (the default assistant workspace is the drive root,
	// so the context dir can sit inside the safe root).
	ContextDir string
	// Unattended marks a session with no human at the UI (Zalo-originated or
	// scheduled). Operations that would normally ask are denied instead.
	Unattended bool
}

// Evaluate applies the approval policy to one hook payload and returns the
// decision the hook should emit. The tier order is fixed: the blocklist beats
// every other rule in every posture, the write boundary beats posture, and
// posture decides the rest.
func Evaluate(in Input, o Options) Output {
	// 1. Security floor: never overridable, interactive or not.
	if command := in.Command(); command != "" {
		if rule, ok := MatchBlocklist(command); ok {
			return Deny(rule.reason(command), rule.Halt)
		}
	}

	// 2. File-writing tools are bounded by the write-safe root and the
	// memory context dir, independent of posture. A write call with no
	// file_path carries nothing to bound, so it falls through to the
	// posture decision below.
	if isWriteTool(in.ToolName) {
		if out, decided := decideWrite(in, o); decided {
			return out
		}
	}

	// 3. Read-only tools are pre-approved in every posture: they cannot
	// mutate state, so prompting for them buys nothing.
	if isReadTool(in.ToolName) {
		return Allow()
	}

	// 4. Shell commands, network fetches, delegation and unknown tools all
	// sit in the ask tier. Interactive sessions get no opinion, so Crush's
	// permission relay asks the user. Unattended sessions deny instead.
	if o.Unattended {
		reason := fmt.Sprintf(
			"gotack-guard: denied by rule %q — an unattended session cannot answer an approval prompt (%s)",
			ruleUnattendedApproval, in.ToolName)
		return Deny(reason, false)
	}
	return None()
}

// decideWrite enforces the write boundary for one write-tool call. The bool
// is false when the call carries no file_path and therefore no boundary to
// enforce; the caller then falls through to the posture decision.
func decideWrite(in Input, o Options) (Output, bool) {
	target := in.FilePath()
	if target == "" {
		return Output{}, false
	}
	abs := resolvePath(in.CWD, target)
	if o.ContextDir != "" && withinPath(o.ContextDir, abs) {
		reason := fmt.Sprintf(
			"gotack-guard: denied by rule %q — writes into the memory context directory are forbidden (path: %s)",
			ruleContextWrite, target)
		return Deny(reason, false), true
	}
	if o.WriteSafeRoot != "" && !withinPath(o.WriteSafeRoot, abs) {
		reason := fmt.Sprintf(
			"gotack-guard: denied by rule %q — file writes are only allowed inside the workspace %s (path: %s)",
			ruleWriteOutsideRoot, o.WriteSafeRoot, target)
		return Deny(reason, false), true
	}
	return Allow(), true
}
