package guard

import (
	"encoding/json"
	"fmt"
)

const (
	ruleContextWrite       = "memory-context-write"
	ruleWriteOutsideRoot   = "write-outside-safe-root"
	ruleUnattendedApproval = "unattended-approval"
	ruleReviewWhitelist    = "background-review-tool-whitelist"
)

type Options struct {
	WriteSafeRoot string

	ContextDir string

	Unattended bool

	Review bool
}

func Evaluate(in Input, o Options) Output {

	if command := in.Command(); command != "" {
		if rule, ok := MatchBlocklist(command); ok {
			return Deny(rule.reason(command), rule.Halt)
		}
	}

	if o.Review {
		if isBackgroundReviewTool(in.ToolName) {
			return injectSkillContext(in, o, Allow())
		}
		reason := fmt.Sprintf(
			"gotack-guard: denied by rule %q — background review may only use memory, skill_view, skill_manage, and local read/search tools (%s)",
			ruleReviewWhitelist, in.ToolName)
		return Deny(reason, false)
	}

	if isWriteTool(in.ToolName) {
		if out, decided := decideWrite(in, o); decided {
			return injectSkillContext(in, o, out)
		}
	}

	if isReadTool(in.ToolName) {
		return injectSkillContext(in, o, Allow())
	}

	if o.Unattended {
		reason := fmt.Sprintf(
			"gotack-guard: denied by rule %q — an unattended session cannot answer an approval prompt (%s)",
			ruleUnattendedApproval, in.ToolName)
		return injectSkillContext(in, o, Deny(reason, false))
	}
	return injectSkillContext(in, o, None())
}

func injectSkillContext(in Input, o Options, out Output) Output {
	if !isSkillTool(in.ToolName) {
		return out
	}
	patch, _ := json.Marshal(map[string]any{
		"_session_id":        in.SessionID,
		"_background_review": o.Review,
	})
	out.Version = outputVersion
	out.UpdatedInput = patch
	return out
}

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
