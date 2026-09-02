// Package guard implements Gotack's approval policy for Crush tool calls.
//
// It is the decision engine behind the cmd/guard executable, which Crush runs
// as its only hook event (PreToolUse). The package is deliberately pure: it
// reads the hook payload, classifies the requested operation against the
// destructive-command blocklist and the graduated approval tiers, and returns
// a hook decision. It also stamps host-owned context into skills MCP calls and
// applies the smaller background-review whitelist. It performs no agent work
// and never prompts.
package guard
