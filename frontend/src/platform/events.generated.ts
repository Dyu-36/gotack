
export const events = {
  changesUpdated: "changes:updated",
  engineStatus: "engine:status",
  permissionRequest: "permission:request",
  promptFiles: "prompt:files",
  questionRequest: "question:request",
  sessionDelta: "session:delta",
  sessionDone: "session:done",
  terminalData: "terminal:data",
  terminalExit: "terminal:exit",
  toolActivity: "tool:activity",
} as const
export type EventName = (typeof events)[keyof typeof events]
