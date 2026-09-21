import { EventsOn } from '../../wailsjs/runtime/runtime'

export type EngineInfo = {
  status: 'stopped' | 'starting' | 'running' | 'error'
  running: boolean
  endpoint: string
  version: string
  owned: boolean
  error?: string
}
export type WorkspaceInfo = { path: string; workspace_id: string; is_default: boolean; trusted: boolean; trust_required: boolean; trust_inherited_from?: string; protected_resources?: string[] }
export type SessionInfo = { id: string; parent_session_id?: string; title: string; message_count: number; cost: number; updated_at: number; is_busy: boolean }
export type AttachmentInfo = { file_name: string; mime_type: string; size: number; content?: string; path?: string }
export type PromptAttachment = { file_name: string; mime_type?: string; content?: string; path?: string }
export type PromptFilePick = { file_name: string; mime_type: string; size: number; path: string }
export type AttachmentLimitsInfo = { max_bytes: number; max_derived_lines: number; max_derived_bytes: number }
export type ToolCallInfo = { id: string; name: string; input?: string; finished: boolean }
export type MessageInfo = { id: string; role: 'user' | 'assistant' | 'system' | 'tool'; text: string; model: string; provider: string; created_at: number; completed_at?: number; attachments?: AttachmentInfo[]; tool_calls?: ToolCallInfo[] }
export type ChangedFileInfo = { path: string; size: number; updated_at: number }
export type ModelCatalogEntry = {
  id: string
  name: string
  context_window?: number
  default_max_tokens?: number
  can_reason: boolean
  supports_vision?: boolean
  reasoning_levels?: string[]
  default_reasoning_effort?: string
  cost_per_1m_in?: number
  cost_per_1m_out?: number
}
export type ProviderCatalogEntry = {
  id: string
  name: string
  type?: string
  api_endpoint?: string
  default_large_model_id?: string
  default_small_model_id?: string
  models: ModelCatalogEntry[]
  configured: boolean
  credential_kind?: 'api_key' | 'oauth'
}
export type ZaloConfigInfo = {
  pairing_expires_at?: number
  enabled: boolean
  paired_chats: string[]
  pairing_code: string
  has_token: boolean
  bot_name?: string
  token_suffix?: string
  running: boolean
}
export type ZaloConfigUpdate = { enabled: boolean; token?: string }
export type ZaloStatusInfo = {
  pairing_expires_at?: number
  running: boolean
  configured: boolean
  bot_name?: string
  pairing_code?: string
  paired_chat_ids: string[]
  token_suffix?: string
  last_error?: string
}
export type ZaloFileRequest = { path: string; chat_id?: string }

const zaloSendableExtension = /\.(?:png|jpe?g|webp|gif|bmp|pdf|xlsx?|csv|docx?|pptx?|txt|zip|mp4)$/i

function isShareableZaloPath(value: string): boolean {
  if (!value || value.length > 1024 || value.includes('\0')) return false
  if (/^(?:[a-z]:|[\\/])/i.test(value)) return false
  if (!zaloSendableExtension.test(value)) return false
  return value.split(/[\\/]+/).every((segment) => segment.length > 0 && segment !== '..')
}

async function sendZaloFile(req: ZaloFileRequest): Promise<string> {
  const path = req.path.trim()
  if (!isShareableZaloPath(path)) {
    throw new Error('Zalo file sharing only accepts workspace-relative paths of sendable file types')
  }
  return call('SendZaloFile', { ...req, path })
}
export type ChatGPTOAuthStatus = { connected: boolean; email?: string; plan?: string; expires_at?: number }
export type ProviderUsageWindow = {
  id: string
  name?: string
  used_percent: number
  remaining_percent: number
  window_seconds?: number
  resets_at?: number
}
export type ProviderUsageInfo = {
  provider_id: string
  provider_name: string
  available: boolean
  plan?: string
  limit_reached: boolean
  windows: ProviderUsageWindow[]
  updated_at: number
  unavailable_reason?: string
}
export type AgentSettingsInfo = { tools: string[]; disabled_tools: string[] }
export type SettingsInfo = {
  theme: string
  provider: string
  credential_provider?: string
  provider_only?: boolean
  model: string
  thinking: string
  api_key?: string
  custom_url: string
}

export type SessionDeltaEvent = { session_id: string; message_id: string; text: string; append: string; seq: number }
export type SessionDoneEvent = { session_id: string; text?: string; error?: string; cancelled?: boolean }
export type SessionUpdatedEvent = { session_id: string; title: string; updated_at: number }
export type ToolActivityEvent = { session_id: string; name: string; input: unknown; finished: boolean; tool_call_id: string }

type BackendApp = {
  BackendReady: () => Promise<boolean>
  GetAutoStart: () => Promise<boolean>
  SetAutoStart: (enabled: boolean) => Promise<void>
  EngineStatus: () => Promise<EngineInfo>
  StartEngine: () => Promise<EngineInfo>
  StopEngine: () => Promise<EngineInfo>
  ReconnectEngine: () => Promise<void>
  SelectWorkspace: () => Promise<string>
  ListRecentWorkspaces: () => Promise<string[]>
  OpenWorkspace: (path: string) => Promise<WorkspaceInfo>
  EnsureAssistantWorkspace: () => Promise<WorkspaceInfo>
  CurrentWorkspace: () => Promise<WorkspaceInfo | null>
  SetWorkspaceTrust: (trusted: boolean) => Promise<WorkspaceInfo>
  ResetWorkspaceTrust: () => Promise<WorkspaceInfo>
  ListSessions: () => Promise<SessionInfo[]>
  CreateSession: (title: string) => Promise<SessionInfo>
  CloneSession: (id: string) => Promise<SessionInfo>
  ForkSession: (id: string, messageID: string) => Promise<SessionInfo>
  CompactSession: (id: string) => Promise<void>
  RenameSession: (id: string, title: string) => Promise<SessionInfo>
  DeleteSession: (id: string) => Promise<void>
  SwitchSession: (id: string) => Promise<void>
  SessionMessages: (id: string) => Promise<MessageInfo[]>
  SendPrompt: (id: string, text: string, attachments: PromptAttachment[]) => Promise<string>
  CancelPrompt: (id: string) => Promise<void>
  PickPromptFiles: () => Promise<PromptFilePick[]>
  AttachmentLimits: () => Promise<AttachmentLimitsInfo>
  OpenGeneratedFile: (path: string) => Promise<void>
  RevealGeneratedFile: (path: string) => Promise<void>
  ChangedFiles: (sessionID: string) => Promise<ChangedFileInfo[]>
  FileDiff: (sessionID: string, path: string) => Promise<string>
  GetSettings: () => Promise<SettingsInfo>
  SaveSettings: (settings: SettingsInfo) => Promise<void>
  GetAgentSettings: () => Promise<AgentSettingsInfo>
  SaveAgentSettings: (disabledTools: string[]) => Promise<AgentSettingsInfo>
  ListProviders: () => Promise<ProviderCatalogEntry[]>
  GetProviderUsage: (providerID: string) => Promise<ProviderUsageInfo>
  DeleteProvider: (providerID: string) => Promise<void>
  LoginChatGPTOAuth: () => Promise<ChatGPTOAuthStatus>
  GetChatGPTOAuthStatus: () => Promise<ChatGPTOAuthStatus>
  LogoutChatGPTOAuth: () => Promise<void>
  CancelChatGPTOAuth: () => Promise<void>
  GetChatGPTOAuthURL: () => Promise<string>
  GetZaloConfig: () => Promise<ZaloConfigInfo>
  SaveZaloConfig: (update: ZaloConfigUpdate) => Promise<ZaloStatusInfo>
  TestZaloConnection: () => Promise<ZaloStatusInfo>
  RemoveZaloToken: () => Promise<ZaloStatusInfo>
  RegenerateZaloPairingCode: () => Promise<ZaloStatusInfo>
  UnpairZaloChat: (chatID: string) => Promise<ZaloStatusInfo>
  ZaloStatus: () => Promise<ZaloStatusInfo>
  SendZaloFile: (req: ZaloFileRequest) => Promise<string>
}

declare global {
  interface Window { go?: { main?: { App?: Partial<BackendApp> } } }
}
function app(): BackendApp | null { return (window.go?.main?.App as BackendApp) ?? null }
import { events, type EventName } from './events.generated'
export { events, type EventName }
export function on<T>(event: EventName | string, handler: (payload: T) => void): () => void {
  const wrapped = (...data: unknown[]) => handler(data[0] as T)
  return EventsOn(event, wrapped)
}
function call<K extends keyof BackendApp>(method: K, ...args: Parameters<BackendApp[K]>): ReturnType<BackendApp[K]> {
  const fn = app()?.[method] as ((...a: unknown[]) => Promise<unknown>) | undefined
  if (!fn) return Promise.reject(new Error(`backend method ${String(method)} unavailable`)) as ReturnType<BackendApp[K]>
  return fn(...args) as ReturnType<BackendApp[K]>
}
export const desktop = {
  on,
  available: () => app() !== null,
  backendReady: async () => app()?.BackendReady ? app()!.BackendReady() : false,
  getAutoStart: () => call('GetAutoStart'), setAutoStart: (enabled: boolean) => call('SetAutoStart', enabled),
  engineStatus: () => call('EngineStatus'), startEngine: () => call('StartEngine'), stopEngine: () => call('StopEngine'), reconnectEngine: () => call('ReconnectEngine'),
  selectWorkspace: () => call('SelectWorkspace'), listRecentWorkspaces: () => call('ListRecentWorkspaces'), openWorkspace: (path: string) => call('OpenWorkspace', path), ensureAssistantWorkspace: () => call('EnsureAssistantWorkspace'), currentWorkspace: () => call('CurrentWorkspace'), setWorkspaceTrust: (trusted: boolean) => call('SetWorkspaceTrust', trusted), resetWorkspaceTrust: () => call('ResetWorkspaceTrust'),
  listSessions: () => call('ListSessions'), createSession: (title: string) => call('CreateSession', title), cloneSession: (id: string) => call('CloneSession', id), forkSession: (id: string, messageID: string) => call('ForkSession', id, messageID), compactSession: (id: string) => call('CompactSession', id), renameSession: (id: string, title: string) => call('RenameSession', id, title), deleteSession: (id: string) => call('DeleteSession', id), switchSession: (id: string) => call('SwitchSession', id), sessionMessages: (id: string) => call('SessionMessages', id), sendPrompt: (id: string, text: string, attachments: PromptAttachment[] = []) => call('SendPrompt', id, text, attachments), cancelPrompt: (id: string) => call('CancelPrompt', id),
  pickPromptFiles: () => call('PickPromptFiles'), attachmentLimits: () => call('AttachmentLimits'),
  openGeneratedFile: (path: string) => call('OpenGeneratedFile', path), revealGeneratedFile: (path: string) => call('RevealGeneratedFile', path),
  changedFiles: (sessionID: string) => call('ChangedFiles', sessionID), fileDiff: (sessionID: string, path: string) => call('FileDiff', sessionID, path),
  getZaloConfig: () => call('GetZaloConfig'), saveZaloConfig: (update: ZaloConfigUpdate) => call('SaveZaloConfig', update), testZaloConnection: () => call('TestZaloConnection'), removeZaloToken: () => call('RemoveZaloToken'), regenerateZaloPairingCode: () => call('RegenerateZaloPairingCode'), unpairZaloChat: (chatID: string) => call('UnpairZaloChat', chatID), zaloStatus: () => call('ZaloStatus'), sendZaloFile: (req: ZaloFileRequest) => sendZaloFile(req),
  getSettings: () => call('GetSettings'), saveSettings: (settings: SettingsInfo) => call('SaveSettings', settings), getAgentSettings: () => call('GetAgentSettings'), saveAgentSettings: (disabledTools: string[]) => call('SaveAgentSettings', disabledTools),
  listProviders: () => call('ListProviders'), getProviderUsage: (providerID: string) => call('GetProviderUsage', providerID), deleteProvider: (providerID: string) => call('DeleteProvider', providerID),
  loginChatGPTOAuth: () => call('LoginChatGPTOAuth'), getChatGPTOAuthStatus: () => call('GetChatGPTOAuthStatus'), logoutChatGPTOAuth: () => call('LogoutChatGPTOAuth'), cancelChatGPTOAuth: () => call('CancelChatGPTOAuth'), getChatGPTOAuthURL: () => call('GetChatGPTOAuthURL'),
}
