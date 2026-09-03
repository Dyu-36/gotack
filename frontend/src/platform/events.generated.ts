// events.generated.ts -- DO NOT EDIT.
// Source: internal/uievents/names.go
// Regenerate with: go run ./internal/uievents/gen/main.go

export const events = {
  changesUpdated: "changes:updated",
  engineStatus: "engine:status",
  permissionRequest: "permission:request",
  promptFiles: "prompt:files",
  sessionDelta: "session:delta",
  sessionDone: "session:done",
  terminalData: "terminal:data",
  terminalExit: "terminal:exit",
  toolActivity: "tool:activity",
} as const
export type EventName = (typeof events)[keyof typeof events]
