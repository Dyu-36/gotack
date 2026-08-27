package uievents

// names.go -- role: the single list of event names shared with the UI.
//
// Renaming an event here is a contract change: update
// docs/contracts/wails-bindings.md in the same commit.

// Event names emitted to the webview via the Wails runtime. The string values
// are what the Svelte side subscribes to; do not change them without updating
// the front-end listener map in the same commit.
const (
	EngineStatus      = "engine:status"
	SessionDelta      = "session:delta"
	SessionDone       = "session:done"
	ToolActivity      = "tool:activity"
	PermissionRequest = "permission:request"
	QuestionRequest   = "question:request"
	ChangesUpdated    = "changes:updated"
	TerminalData      = "terminal:data"
	TerminalExit      = "terminal:exit"
)
