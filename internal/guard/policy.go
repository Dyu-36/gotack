package guard

// policy.go -- role: evaluate one tool call and return the hook decision.
//
// Stage 1 posture (deny-only): the guard hard-denies commands that trip the
// unrecoverable-command blocklist and passes every other tool call through
// untouched (no opinion). Shipping the floor before any approval-mode change
// raises the security bar without introducing prompts an unattended session
// cannot answer (risk R6).

// Evaluate applies the approval policy to one hook payload and returns the
// decision the hook should emit. In stage 1 the only enforced rule is the
// destructive-command blocklist; all other operations fall through.
func Evaluate(in Input) Output {
	if command := in.Command(); command != "" {
		if rule, ok := MatchBlocklist(command); ok {
			return Deny(rule.reason(command), rule.Halt)
		}
	}
	return None()
}
