import { desktop, type EngineInfo, type PermissionRequestPayload as Envelope } from '../../platform/desktop'
import { ChatMessage, type ChatAttachment, type Conversation, type ReasoningEffort, type SessionSummary } from './types.svelte'
import { catalog, REASONING_EFFORT_OPTIONS } from './catalog.svelte'
import { createEngineState } from './live-conversation-engine.svelte'
import { createMessageState } from './live-conversation-messages.svelte'
import { createPermissionState } from './live-conversation-permissions.svelte'

const SESSION_MEMORY_PREFIX = 'gotack.active-session:'
const DEFAULT_WORKSPACE_LABEL = 'C:\\'

export function createLiveConversationState() {

  let conversations = $state<Conversation[]>([])
  let activeId = $state('')
  let input = $state('')
  let attachments = $state<ChatAttachment[]>([])
  let workspace = $state(DEFAULT_WORKSPACE_LABEL)
  let backendReady = $state(false)
  let engine = $state<EngineInfo | null>(null)
  let error = $state('')
  let errorTimer: number | undefined
  let permission = $state<Envelope | null>(null)
  let streamingText = $state('')
  let provider = $state('')
  let model = $state('')
  let modelLabel = $state('Model mặc định')
  let thinking = $state<ReasoningEffort>('high')
  let apiKey = $state('')
  let customUrl = $state('')

  const errorText = (cause: unknown) => cause instanceof Error ? cause.message : String(cause)
  const reportError = (cause: unknown, prefix = '') => {
    const message = `${prefix}${prefix ? ': ' : ''}${errorText(cause)}`
    if (errorTimer !== undefined) window.clearTimeout(errorTimer)
    error = message
    errorTimer = window.setTimeout(() => {
      if (error === message) error = ''
      errorTimer = undefined
    }, 5000)
  }
  const clearError = () => {
    if (errorTimer !== undefined) window.clearTimeout(errorTimer)
    errorTimer = undefined
    error = ''
  }
  const updateConversation = (id: string, fn: (c: Conversation) => Conversation) => {
    conversations = conversations.map((c) => c.id === id ? fn(c) : c)
  }
  const rememberSession = (id: string) => {
    if (id) localStorage.setItem(`${SESSION_MEMORY_PREFIX}${workspace}`, id)
  }

  const messages = createMessageState({
    conversations: { get value() { return conversations }, set value(v) { conversations = v } },
    activeId: { get value() { return activeId }, set value(v) { activeId = v } },

    input: { get value() { return input }, set value(v) { input = v } },
    attachments: { get value() { return attachments }, set value(v) { attachments = v } },
    workspace: { get value() { return workspace }, set value(v) { workspace = v } },
    streamingText: { get value() { return streamingText }, set value(v) { streamingText = v } },
    reportError, clearError, updateConversation, rememberSession,
    applyLoadedSelection: (providerID, modelID) => engineState.applyLoadedSelection(providerID, modelID),
    waitForReady: () => engineState.waitForReady(),
    waitForSelection: () => engineState.waitForSelection(),
  })

  const engineState = createEngineState({
    conversations: { get value() { return conversations } },
    backendReady: { get value() { return backendReady }, set value(v) { backendReady = v } },
    engine: { get value() { return engine }, set value(v) { engine = v } },
    error: { get value() { return error }, set value(v) { error = v } },
    permission: { get value() { return permission }, set value(v) { permission = v } },
    streamingText: { get value() { return streamingText }, set value(v) { streamingText = v } },
    provider: { get value() { return provider }, set value(v) { provider = v } },
    model: { get value() { return model }, set value(v) { model = v } },
    modelLabel: { get value() { return modelLabel }, set value(v) { modelLabel = v } },
    thinking: { get value() { return thinking }, set value(v) { thinking = v } },
    apiKey: { get value() { return apiKey }, set value(v) { apiKey = v } },
    customUrl: { get value() { return customUrl }, set value(v) { customUrl = v } },
    activeId: { get value() { return activeId } },
    reportError, clearError, updateConversation,
    ensureWorkspace: () => messages.ensureWorkspace(),
    reloadMessages: (id: string) => messages.loadMessages(id),

    attachPaths: (picks) => messages.attachPaths(picks),
  })

  const permissions = createPermissionState({
    permission: { get value() { return permission }, set value(v) { permission = v } },
    reportError,
  })

  return {
    get sessions(): SessionSummary[] {
      return conversations.map(({ id, title, updatedAt, pinned, status: s }) => ({ id, title, updatedAt, pinned, streaming: s === 'streaming' }))
    },
    get activeId() { return activeId },
    get active() { return conversations.find((item) => item.id === activeId) },
    get input() { return input },
    get attachments() { return attachments },
    get workspace() { return workspace },
    get backendReady() { return backendReady },
    get engine() { return engine },
    get error() { return error },
    get permission() { return permission },
    get streamingText() { return streamingText },
    get provider() { return provider },
    get model() { return model },
    get modelLabel() { return modelLabel },
    get thinking() { return thinking },
    get thinkingLabel() {
      const selected = catalog.configuredModels.find((m) => m.id === model && (!provider || m.providerId === provider))
      if (selected?.can_reason && !selected.reasoning_levels?.length) return thinking === 'none' ? 'Think: Off' : 'Think: On'
      const opt = REASONING_EFFORT_OPTIONS.find((o) => o.id === thinking)
      return opt ? `Think: ${opt.short}` : 'Think: Auto'
    },
    get apiKey() { return apiKey },
    get customUrl() { return customUrl },
    get permissionSecondsLeft() { return permissions.permissionSecondsLeft.value },
    get permissionExpired() { return permissions.permissionExpired.value },

    setInput: (v: string) => { input = v },
    attachFiles: (files: File[]) => messages.attachFiles(files),

    get hasFilePicker() { return desktop.available() },
    pickFiles: () => messages.pickFiles(),
    removeAttachment: (id: string) => messages.removeAttachment(id),
    setModel: (next: string, label?: string, providerID?: string) => engineState.setModel(next, label, providerID),
    setThinking: (value: ReasoningEffort) => engineState.setThinking(value),
    init: () => {
      console.log('init called, desktop available:', desktop.available())
      if (!desktop.available()) {
        try {
          console.log('running browser mock init')
          backendReady = true
          modelLabel = 'Claude 3.7 Sonnet'
          const demoId = 'session-demo-1'
          activeId = demoId

          const m1 = new ChatMessage('msg-1', 'user', Date.now() - 60000)
          m1.content = 'Xin chào, hãy phân tích cho tôi cấu trúc dự án và hiển thị ví dụ mã nguồn.'

          const mTool = new ChatMessage('msg-tool-1', 'assistant', Date.now() - 50000)
          mTool.kind = 'tool'
          mTool.toolName = 'list_dir'
          mTool.toolFinished = true
          mTool.content = 'Scanning workspace D:/gotack...'

          const m2 = new ChatMessage('msg-2', 'assistant', Date.now() - 40000)
          m2.content = `Chào bạn! Dưới đây là phân tích cấu trúc dự án và ví dụ cụ thể:

### 1. Cấu trúc tổng quan
Dự án được chia thành các phần chính:
- **Frontend**: Svelte 5 + Tailwind CSS v4
- **Backend (Desktop host)**: Go + Wails v2
- **Engine**: REST + SSE communication

### 2. Bảng so sánh các thành phần
| Thành phần | Vai trò | Công nghệ |
| :--- | :--- | :--- |
| UI Frame | Giao diện người dùng | Svelte 5, Tailwind CSS |
| Desktop Host | Cầu nối hệ thống | Wails v2 (Go) |
| Agent Engine | Xử lý mô hình AI & tools | REST / SSE |

### 3. Ví dụ mã cấu hình
\`\`\`typescript
export function setupClient() {
  console.log("Gotack Client initialized!");
  return { status: "ready", timestamp: Date.now() };
}
\`\`\`

Bạn cần tôi hỗ trợ kiểm tra hay tinh chỉnh thêm phần nào không?`

          const m3 = new ChatMessage('msg-3', 'user', Date.now() - 20000)
          m3.content = 'Hãy kiểm tra vị trí của khung chat và độ cân đối của các tin nhắn.'

          const m4 = new ChatMessage('msg-4', 'assistant', Date.now() - 5000)
          m4.content = 'Tôi đang hiển thị câu trả lời mẫu để bạn kiểm tra vị trí tin nhắn của người dùng và mô hình trên giao diện.'

          conversations = [{
            id: demoId,
            title: 'Hội thoại mẫu kiểm tra giao diện',
            updatedAt: Date.now(),
            pinned: false,
            status: 'idle',
            messages: [m1, mTool, m2, m3, m4],
          }]
          console.log('conversations set successfully, count:', conversations.length)
        } catch (e) {
          console.error('ERROR in mock init:', e)
        }
        return Promise.resolve()
      }
      return engineState.init()
    },
    destroy: () => { clearError(); engineState.destroy() },
    pickWorkspace: () => messages.pickWorkspace(),
    create: () => messages.create(),
    select: (id: string) => messages.select(id),
    send: () => messages.send(),
    cancel: () => messages.cancel(),
    rename: (id: string, title: string) => messages.rename(id, title),
    delete: (id: string) => messages.remove(id),
    answerPermission: (decision: 'allow' | 'allow_session' | 'deny') => permissions.answerPermission(decision),
    loadSettings: () => engineState.loadSettings(),
    saveSettings: (s: { theme: string; provider: string; credential_provider?: string; provider_only?: boolean; model: string; thinking: string; api_key: string; custom_url: string }) => engineState.saveSettings(s),
  }
}
