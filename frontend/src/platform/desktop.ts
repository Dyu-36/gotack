// desktop.ts -- role: the only path from UI to host. Every window.go call and
// every host event subscription goes through this module. Method names mirror
// docs/contracts/wails-bindings.md.

type EngineInfo = {
  status: 'stopped' | 'starting' | 'running' | 'error'
  running: boolean
  endpoint: string
  version: string
  owned: boolean
  error?: string
}

type WorkspaceInfo = {
  path: string
  workspace_id: string
}

type SessionInfo = {
  id: string
  title: string
  message_count: number
  cost: number
  updated_at: number
  is_busy: boolean
}

type MessageInfo = {
  id: string
  role: 'user' | 'assistant' | 'system' | 'tool'
  text: string
  created_at: number
}

type ChangedFileInfo = {
  path: string
  size: number
  updated_at: number
}

export type SettingsInfo = {
  theme: string
  autostart_engine: boolean
  provider: string
  model: string
  thinking: string
  api_key: string
  custom_url: string
}

type PermissionRequestEvent = {
  id: string
  session_id: string
  tool_call_id: string
  tool_name: string
  description: string
  action: string
  params: unknown
  path: string
}

type QuestionChoice = { id: string; label: string; description?: string }

type QuestionRequestEvent = {
  id: string
  session_id: string
  tool_call_id: string
  questions: Array<{
    id: string
    type: string
    label?: string
    question: string
    description?: string
    choices?: QuestionChoice[]
  }>
  confirm_title?: string
  confirm_description?: string
}

export type SessionDeltaEvent = {
  session_id: string
  message_id: string
  text: string
  // Suffix of `text` that the UI has not seen yet, relative to the previous
  // delta for the same message_id. Equal to `text` on the first delta for a
  // message; empty after the last one. UI implementations can either keep
  // replacing `text` in place (current contract) or switch to appending
  // `append` to a local buffer; both stay correct because the host still
  // sends the full text on every delta.
  append: string
}

export type SessionDoneEvent = {
  session_id: string
  text?: string
  error?: string
  cancelled?: boolean
}

type BackendApp = {
  BackendReady: () => Promise<boolean>
  EngineStatus: () => Promise<EngineInfo>
  StartEngine: () => Promise<EngineInfo>
  StopEngine: () => Promise<EngineInfo>
  ReconnectEngine: () => Promise<void>
  ListRecentWorkspaces: () => Promise<string[]>
  OpenWorkspace: (path: string) => Promise<WorkspaceInfo>
  CurrentWorkspace: () => Promise<WorkspaceInfo | null>
  ListSessions: () => Promise<SessionInfo[]>
  CreateSession: (title: string) => Promise<SessionInfo>
  SwitchSession: (id: string) => Promise<void>
  SessionMessages: (id: string) => Promise<MessageInfo[]>
  SendPrompt: (id: string, text: string) => Promise<string>
  CancelPrompt: (id: string) => Promise<void>
  AnswerPermission: (requestID: string, decision: 'allow' | 'allow_session' | 'deny') => Promise<boolean>
  AnswerQuestion: (
    requestID: string,
    answers: Array<{
      request_id: string
      selected_ids?: string[]
      fill_in_text?: string
      yes?: boolean | null
    }>,
  ) => Promise<boolean>
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
    go?: {
      main?: {
        App?: Partial<BackendApp>
      }
    }
    runtime?: {
      EventsOn: (name: string, cb: (...data: unknown[]) => void) => void
      EventsOff: (name: string, ...additional: string[]) => void
    }
  }
}

function app(): BackendApp | null {
  const bound = window.go?.main?.App
  return (bound as BackendApp) ?? null
}

// Host event names, mirroring internal/uievents/names.go.
export const events = {
  engineStatus: 'engine:status',
  sessionDelta: 'session:delta',
  sessionDone: 'session:done',
  toolActivity: 'tool:activity',
  permissionRequest: 'permission:request',
  questionRequest: 'question:request',
  changesUpdated: 'changes:updated',
  terminalData: 'terminal:data',
  terminalExit: 'terminal:exit',
} as const

export type EventName = (typeof events)[keyof typeof events]

// on subscribes to a host event; returns an unsubscribe function.
// Uses the Wails runtime injected as window.runtime.
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
  available(): boolean {
    return app() !== null
  },

  async backendReady(): Promise<boolean> {
    const ready = app()?.BackendReady
    if (!ready) return false
    return ready()
  },

  engineStatus: () => call('EngineStatus'),
  startEngine: () => call('StartEngine'),
  stopEngine: () => call('StopEngine'),
  reconnectEngine: () => call('ReconnectEngine'),

  listRecentWorkspaces: () => call('ListRecentWorkspaces'),
  openWorkspace: (path: string) => call('OpenWorkspace', path),
  currentWorkspace: () => call('CurrentWorkspace'),

  listSessions: () => call('ListSessions'),
  createSession: (title: string) => call('CreateSession', title),
  switchSession: (id: string) => call('SwitchSession', id),
  sessionMessages: (id: string) => call('SessionMessages', id),
  sendPrompt: (id: string, text: string) => call('SendPrompt', id, text),
  cancelPrompt: (id: string) => call('CancelPrompt', id),

  answerPermission: (requestID: string, decision: 'allow' | 'allow_session' | 'deny') =>
    call('AnswerPermission', requestID, decision),
  answerQuestion: (
    requestID: string,
    answers: Array<{ request_id: string; selected_ids?: string[]; fill_in_text?: string; yes?: boolean | null }>,
  ) => call('AnswerQuestion', requestID, answers),

  changedFiles: (sessionID: string) => call('ChangedFiles', sessionID),
  fileDiff: (sessionID: string, path: string) => call('FileDiff', sessionID, path),

  openTerminal: (cwd: string) => call('OpenTerminal', cwd),
  writeTerminal: (id: string, data: string) => call('WriteTerminal', id, data),
  resizeTerminal: (id: string, cols: number, rows: number) => call('ResizeTerminal', id, cols, rows),
  closeTerminal: (id: string) => call('CloseTerminal', id),

  getSettings: () => call('GetSettings'),
  saveSettings: (settings: SettingsInfo) => call('SaveSettings', settings),
}
