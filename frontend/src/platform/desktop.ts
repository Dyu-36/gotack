// desktop.ts -- role: the only path from UI to host. Every window.go call and
// every host event subscription goes through this module. Method names mirror
// docs/contracts/wails-bindings.md.

export type EngineInfo = {
  status: 'stopped' | 'starting' | 'running' | 'error'
  running: boolean
  endpoint: string
  version: string
  owned: boolean
  error?: string
}

export type WorkspaceInfo = { path: string; workspace_id: string }
export type SessionInfo = { id: string; title: string; message_count: number; cost: number; updated_at: number; is_busy: boolean }
export type MessageInfo = { id: string; role: 'user' | 'assistant' | 'system' | 'tool'; text: string; created_at: number }
export type ChangedFileInfo = { path: string; size: number; updated_at: number }

export type SettingsInfo = {
  theme: string
  autostart_engine: boolean
  provider: string
  model: string
  thinking: string
  api_key: string
  custom_url: string
}

export type PermissionRequestEvent = {
  id: string
  session_id: string
  tool_call_id: string
  tool_name: string
  description: string
  action: string
  params: unknown
  path: string
}

export type QuestionChoice = { id: string; label: string; description?: string }
export type QuestionRequestEvent = {
  id: string
  session_id: string
  tool_call_id: string
  questions: Array<{ id: string; type: string; label?: string; question: string; description?: string; choices?: QuestionChoice[] }>
  confirm_title?: string
  confirm_description?: string
}

export type SessionDeltaEvent = { session_id: string; message_id: string; text: string; append: string }
export type SessionDoneEvent = { session_id: string; text?: string; error?: string; cancelled?: boolean }
export type ToolActivityEvent = { session_id: string; name: string; input: unknown; finished: boolean; tool_call_id: string }
export type TerminalDataEvent = { id: string; data: string }
export type TerminalExitEvent = { id: string; code?: number; error?: string }

type BackendApp = {
  BackendReady: () => Promise<boolean>
  EngineStatus: () => Promise<EngineInfo>
  StartEngine: () => Promise<EngineInfo>
  StopEngine: () => Promise<EngineInfo>
  ReconnectEngine: () => Promise<void>
  SelectWorkspace: () => Promise<string>
  ListRecentWorkspaces: () => Promise<string[]>
  OpenWorkspace: (path: string) => Promise<WorkspaceInfo>
  CurrentWorkspace: () => Promise<WorkspaceInfo | null>
  ListSessions: () => Promise<SessionInfo[]>
  CreateSession: (title: string) => Promise<SessionInfo>
  RenameSession: (id: string, title: string) => Promise<SessionInfo>
  DeleteSession: (id: string) => Promise<void>
  SwitchSession: (id: string) => Promise<void>
  SessionMessages: (id: string) => Promise<MessageInfo[]>
  SendPrompt: (id: string, text: string) => Promise<string>
  CancelPrompt: (id: string) => Promise<void>
  AnswerPermission: (requestID: string, decision: 'allow' | 'allow_session' | 'deny') => Promise<boolean>
  AnswerQuestion: (requestID: string, answers: Array<{ request_id: string; selected_ids?: string[]; fill_in_text?: string; yes?: boolean | null }>) => Promise<boolean>
  ChangedFiles: (sessionID: string) => Promise<ChangedFileInfo[]>
  FileDiff: (sessionID: string, path: string) => Promise<string>
  OpenTerminal: (cwd: string) => Promise<string>
  WriteTerminal: (id: string, data: string) => Promise<void>
  ResizeTerminal: (id: string, cols: number, rows: number) => Promise<void>
  CloseTerminal: (id: string) => Promise<void>
  GetSettings: () => Promise<SettingsInfo>
  SaveSettings: (settings: SettingsInfo) => Promise<void>
}

declare global {
  interface Window {
    go?: { main?: { App?: Partial<BackendApp> } }
    runtime?: { EventsOn: (name: string, cb: (...data: unknown[]) => void) => void; EventsOff: (name: string, ...additional: string[]) => void }
  }
}

function app(): BackendApp | null {
  const bound = window.go?.main?.App
  return (bound as BackendApp) ?? null
}

export const events = {
  engineStatus: 'engine:status', sessionDelta: 'session:delta', sessionDone: 'session:done', toolActivity: 'tool:activity',
  permissionRequest: 'permission:request', questionRequest: 'question:request', changesUpdated: 'changes:updated', terminalData: 'terminal:data', terminalExit: 'terminal:exit',
} as const
export type EventName = (typeof events)[keyof typeof events]

export function on<T>(event: EventName, handler: (payload: T) => void): () => void {
  const rt = window.runtime
  if (!rt) return () => {}
  const wrapped = (...data: unknown[]) => handler(data[0] as T)
  rt.EventsOn(event, wrapped)
  return () => rt.EventsOff(event)
}

function call<K extends keyof BackendApp>(method: K, ...args: Parameters<BackendApp[K]>): ReturnType<BackendApp[K]> {
  const fn = app()?.[method] as ((...a: unknown[]) => Promise<unknown>) | undefined
  if (!fn) return Promise.reject(new Error(`backend method ${String(method)} unavailable`)) as ReturnType<BackendApp[K]>
  return fn(...args) as ReturnType<BackendApp[K]>
}

export const desktop = {
  available: () => app() !== null,
  backendReady: async () => app()?.BackendReady ? app()!.BackendReady() : false,
  engineStatus: () => call('EngineStatus'), startEngine: () => call('StartEngine'), stopEngine: () => call('StopEngine'), reconnectEngine: () => call('ReconnectEngine'),
  selectWorkspace: () => call('SelectWorkspace'), listRecentWorkspaces: () => call('ListRecentWorkspaces'), openWorkspace: (path: string) => call('OpenWorkspace', path), currentWorkspace: () => call('CurrentWorkspace'),
  listSessions: () => call('ListSessions'), createSession: (title: string) => call('CreateSession', title), renameSession: (id: string, title: string) => call('RenameSession', id, title), deleteSession: (id: string) => call('DeleteSession', id), switchSession: (id: string) => call('SwitchSession', id), sessionMessages: (id: string) => call('SessionMessages', id), sendPrompt: (id: string, text: string) => call('SendPrompt', id, text), cancelPrompt: (id: string) => call('CancelPrompt', id),
  answerPermission: (requestID: string, decision: 'allow' | 'allow_session' | 'deny') => call('AnswerPermission', requestID, decision),
  answerQuestion: (requestID: string, answers: Array<{ request_id: string; selected_ids?: string[]; fill_in_text?: string; yes?: boolean | null }>) => call('AnswerQuestion', requestID, answers),
  changedFiles: (sessionID: string) => call('ChangedFiles', sessionID), fileDiff: (sessionID: string, path: string) => call('FileDiff', sessionID, path),
  openTerminal: (cwd: string) => call('OpenTerminal', cwd), writeTerminal: (id: string, data: string) => call('WriteTerminal', id, data), resizeTerminal: (id: string, cols: number, rows: number) => call('ResizeTerminal', id, cols, rows), closeTerminal: (id: string) => call('CloseTerminal', id),
  getSettings: () => call('GetSettings'), saveSettings: (settings: SettingsInfo) => call('SaveSettings', settings),
}
