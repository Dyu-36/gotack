package guard

import (
	"encoding/json"
	"fmt"
)

const (
	ruleContextWrite       = "memory-context-write"
	ruleWriteOutsideRoot   = "write-outside-safe-root"
	ruleUnattendedApproval = "unattended-approval"
	ruleUnattendedQuestion = "unattended-question"
	ruleReviewWhitelist    = "background-review-tool-whitelist"
)

type Options struct {
	WriteSafeRoot string
	ContextDir   string
	Unattended   bool
	Review       bool
}

func Evaluate(in Input, o Options) Output {
	if command := in.Command(); command != "" {
		if rule, ok := MatchBlocklist(command); ok {
			return Deny(rule.reason(command), rule.Halt)
		}
	}
	if o.Review {
		if isBackgroundReviewTool(in.ToolName) {
			return Allow()
		}
		return Deny(fmt.Sprintf("gotack-guard: denied by rule %q - personal memory review may only use the memory tool (%s)", ruleReviewWhitelist, in.ToolName), false)
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
		if in.ToolName == "question" {
			reason := fmt.Sprintf("gotack-guard: denied by rule %q - interactive questions are disabled for unattended sessions; ask for the missing information directly in the assistant response, end the turn, and wait for the user's next message", ruleUnattendedQuestion)
			return injectSkillContext(in, o, Deny(reason, false))
		}
		reason := fmt.Sprintf("gotack-guard: denied by rule %q - an unattended session cannot answer an approval prompt (%s)", ruleUnattendedApproval, in.ToolName)
		return injectSkillContext(in, o, Deny(reason, false))
	}
	return injectSkillContext(in, o, None())
}

func injectSkillContext(in Input, o Options, out Output) Output {
	if !isSkillTool(in.ToolName) {
		return out
	}
	patch, _ := json.Marshal(map[string]any{
		"_session_id": in.SessionID, "_background_review": o.Review,
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
		reason := fmt.Sprintf("gotack-guard: denied by rule %q - writes into the personal context directory are forbidden; use memory instead (path: %s)", ruleContextWrite, target)
		return Deny(reason, false), true
	}
	if o.WriteSafeRoot != "" && !withinPath(o.WriteSafeRoot, abs) {
		reason := fmt.Sprintf("gotack-guard: denied by rule %q - file writes are only allowed inside the workspace %s (path: %s)", ruleWriteOutsideRoot, o.WriteSafeRoot, target)
		return Deny(reason, false), true
	}
	return Allow(), true
}
