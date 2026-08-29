import { toast } from 'svelte-sonner'
import type { Conversation, Message, ModelType, ReasoningEffort, SessionSummary } from './types'
import { catalog, REASONING_EFFORT_OPTIONS } from './catalog.svelte'
import {
  desktop,
  events,
  on,
  type EngineInfo,
  type PermissionRequestEvent,
  type QuestionRequestEvent,
  type SessionDeltaEvent,
  type SessionDoneEvent,
  type ToolActivityEvent,
} from '../../platform/desktop'

const NEW_CONVERSATION_TITLE = 'Hội thoại mới'
const SESSION_MEMORY_PREFIX = 'gotack.active-session:'
const RECONNECT_MAX_MS = 30_000

type QuestionAnswer = { request_id: string; selected_ids?: string[]; fill_in_text?: string; yes?: boolean | null }
type SettingsPayload = { theme: string; autostart_engine: boolean; provider: string; model: string; small_model: string; thinking: string; api_key: string; custom_url: string }

export function createLiveConversationState() {
  let conversations = $state<Conversation[]>([])
  let activeId = $state('')
  let input = $state('')
  let workspace = $state('Chọn thư mục...')
  let backendReady = $state(false)
  let engine = $state<EngineInfo | null>(null)
  let error = $state('')
  let permission = $state<PermissionRequestEvent | null>(null)
  let question = $state<QuestionRequestEvent | null>(null)

  let provider = $state('')
  let model = $state('')
  let modelLabel = $state('Model mặc định')
  let smallModel = $state('')
  let thinking = $state<ReasoningEffort>('high')
  let apiKey = $state('')
  let customUrl = $state('')
  let autostartEngine = $state(true)

  let unsubscribers: Array<() => void> = []
  let reconnectTimer: number | undefined
  let reconnectAttempt = 0
  let destroyed = false

  const activeConversation = () => conversations.find((item) => item.id === activeId)
  const updateConversation = (id: string, fn: (c: Conversation) => Conversation) => {
    conversations = conversations.map((c) => c.id === id ? fn(c) : c)
  }

  const errorText = (cause: unknown) => cause instanceof Error ? cause.message : String(cause)
  const reportError = (cause: unknown, prefix = '') => {
    const message = `${prefix}${prefix ? ': ' : ''}${errorText(cause)}`
    error = message
    toast.error(message)
  }
  const clearError = () => { error = '' }

  const memoryKey = () => `${SESSION_MEMORY_PREFIX}${workspace}`
  const rememberSession = (id: string) => {
    if (workspace !== 'Chọn thư mục...' && id) localStorage.setItem(memoryKey(), id)
  }

  const loadMessages = async (id: string) => {
    const rows = await desktop.sessionMessages(id)
    const messages: Message[] = rows
      .filter((m) => m.role === 'user' || m.role === 'assistant')
      .map((m) => ({ id: m.id, role: m.role as 'user' | 'assistant', content: m.text, kind: 'message' }))
    updateConversation(id, (c) => ({ ...c, messages }))
  }

  const loadSessions = async () => {
    const rows = await desktop.listSessions()
    conversations = rows.map((s) => ({
      id: s.id,
      title: s.title || NEW_CONVERSATION_TITLE,
      updatedAt: s.updated_at || Date.now(),
      pinned: false,
      status: s.is_busy ? 'streaming' : 'idle',
      messages: [],
    }))
    if (!conversations.length) {
      const created = await desktop.createSession(NEW_CONVERSATION_TITLE)
      conversations = [{ id: created.id, title: created.title || NEW_CONVERSATION_TITLE, updatedAt: created.updated_at || Date.now(), pinned: false, status: 'idle', messages: [] }]
    }
    const remembered = localStorage.getItem(memoryKey())
    activeId = conversations.some((c) => c.id === remembered) ? remembered! : conversations[0].id
    rememberSession(activeId)
    await desktop.switchSession(activeId)
    await loadMessages(activeId)
  }

  const attachWorkspace = async (path: string) => {
    workspace = path
    await loadSessions()
    await catalog.refresh()
    applyLoadedSelection()
    clearError()
  }

  const openWorkspacePath = async (path: string) => {
    if (!path) return
    const opened = await desktop.openWorkspace(path)
    await attachWorkspace(opened.path)
  }

  const ensureWorkspace = async () => {
    const current = await desktop.currentWorkspace().catch(() => null)
    if (current?.path) {
      await attachWorkspace(current.path)
      return
    }
    const recent = await desktop.listRecentWorkspaces().catch(() => [])
    if (recent.length) await openWorkspacePath(recent[0])
  }

  const clearReconnect = () => {
    if (reconnectTimer !== undefined) window.clearTimeout(reconnectTimer)
    reconnectTimer = undefined
  }

  const scheduleReconnect = () => {
    if (destroyed || !backendReady || reconnectTimer !== undefined) return
    const delay = Math.min(RECONNECT_MAX_MS, 750 * (2 ** reconnectAttempt))
    reconnectAttempt += 1
    reconnectTimer = window.setTimeout(() => {
      reconnectTimer = undefined
      void desktop.reconnectEngine().catch((cause) => {
        reportError(cause, 'Reconnect failed')
        scheduleReconnect()
      })
    }, delay)
  }

  const handleEngine = async (info: EngineInfo) => {
    engine = info
    if (info.running) {
      reconnectAttempt = 0
      clearReconnect()
      clearError()
      await ensureWorkspace().catch((cause) => reportError(cause, 'Restore workspace'))
      return
    }
    catalog.reset()
    if (info.error) error = info.error
    if (info.status === 'error') scheduleReconnect()
  }

  const subscribe = () => {
    unsubscribers.forEach((off) => off())
    unsubscribers = [
      on<EngineInfo>(events.engineStatus, (event) => void handleEngine(event)),
      on<SessionDeltaEvent>(events.sessionDelta, (event) => {
        updateConversation(event.session_id, (c) => {
          const index = c.messages.findIndex((m) => m.id === event.message_id)
          const next = [...c.messages]
          if (index >= 0) next[index] = { ...next[index], content: event.text, kind: 'message' }
          else next.push({ id: event.message_id, role: 'assistant', content: event.text, kind: 'message' })
          return { ...c, status: 'streaming', updatedAt: Date.now(), messages: next }
        })
      }),
      on<ToolActivityEvent>(events.toolActivity, (event) => {
        updateConversation(event.session_id, (c) => {
          const id = `tool:${event.tool_call_id || event.name}`
          const index = c.messages.findIndex((m) => m.id === id)
          let content = ''
          try {
            const raw = typeof event.input === 'string' ? event.input : JSON.stringify(event.input)
            content = raw.length > 180 ? `${raw.slice(0, 180)}…` : raw
          } catch { content = '' }
          const tool: Message = { id, role: 'assistant', kind: 'tool', toolName: event.name, toolFinished: event.finished, content }
          const next = [...c.messages]
          if (index >= 0) next[index] = tool
          else next.push(tool)
          return { ...c, messages: next }
        })
      }),
      on<SessionDoneEvent>(events.sessionDone, (event) => {
        updateConversation(event.session_id, (c) => ({ ...c, status: 'idle', updatedAt: Date.now() }))
        if (event.error) reportError(event.error, 'Agent run')
      }),
      on<PermissionRequestEvent>(events.permissionRequest, (event) => (permission = event)),
      on<QuestionRequestEvent>(events.questionRequest, (event) => (question = event)),
    ]
  }

  // applyLoadedSelection resolves the stored provider/model against the live
  // catalog once it is available. Unknown stored ids keep their raw value so
  // the truth stays visible instead of silently resetting.
  const applyLoadedSelection = () => {
    if (provider) modelLabel = catalog.modelName(model, provider) ?? model
  }

  const loadSettings = async () => {
    const s = await desktop.getSettings().catch(() => null)
    if (!s) return
    if (s.provider) provider = s.provider
    if (s.model) model = s.model
    if (s.small_model) smallModel = s.small_model
    if (s.thinking) thinking = s.thinking as ReasoningEffort
    autostartEngine = s.autostart_engine
    apiKey = ''
    customUrl = s.custom_url ?? ''
    if (catalog.status === 'ready') applyLoadedSelection()
  }

  const init = async () => {
    destroyed = false
    backendReady = await desktop.backendReady().catch(() => false)
    if (!backendReady) {
      reportError('Wails backend unavailable')
      return
    }
    subscribe()
    await loadSettings()
    try {
      const status = await desktop.startEngine()
      engine = status
      if (status.running) await ensureWorkspace()
    } catch (cause) {
      reportError(cause, 'Start Crush')
      scheduleReconnect()
    }
  }

  const destroy = () => {
    destroyed = true
    clearReconnect()
    unsubscribers.forEach((off) => off())
    unsubscribers = []
  }

  const pickWorkspace = async () => {
    try {
      const path = await desktop.selectWorkspace()
      if (path) await openWorkspacePath(path)
    } catch (cause) { reportError(cause, 'Open workspace') }
  }

  const create = async () => {
    if (!engine?.running || workspace === 'Chọn thư mục...') return
    try {
      const s = await desktop.createSession(NEW_CONVERSATION_TITLE)
      const c: Conversation = { id: s.id, title: s.title || NEW_CONVERSATION_TITLE, updatedAt: s.updated_at || Date.now(), pinned: false, status: 'idle', messages: [] }
      conversations = [c, ...conversations]
      activeId = c.id
      rememberSession(activeId)
      await desktop.switchSession(c.id)
    } catch (cause) { reportError(cause, 'Create session') }
  }

  const select = async (id: string) => {
    try {
      activeId = id
      rememberSession(id)
      input = ''
      await desktop.switchSession(id)
      await loadMessages(id)
      clearError()
    } catch (cause) { reportError(cause, 'Switch session') }
  }

  const send = async () => {
    const text = input.trim()
    const current = activeConversation()
    if (!text || !current || current.status === 'streaming' || !backendReady) return
    input = ''
    updateConversation(current.id, (c) => ({ ...c, status: 'streaming', messages: [...c.messages, { role: 'user', content: text, kind: 'message' }] }))
    try {
      await desktop.sendPrompt(current.id, text)
    } catch (cause) {
      reportError(cause, 'Send prompt')
      updateConversation(current.id, (c) => ({ ...c, status: 'idle' }))
    }
  }

  const cancel = async () => {
    const current = activeConversation()
    if (!current || current.status !== 'streaming') return
    try { await desktop.cancelPrompt(current.id) } catch (cause) { reportError(cause, 'Cancel prompt') }
  }

  const rename = async (id: string, title: string) => {
    const clean = title.trim()
    if (!clean) return
    try {
      const saved = await desktop.renameSession(id, clean)
      updateConversation(id, (c) => ({ ...c, title: saved.title || clean, updatedAt: saved.updated_at || Date.now() }))
    } catch (cause) { reportError(cause, 'Rename session') }
  }

  const remove = async (id: string) => {
    try {
      await desktop.deleteSession(id)
      conversations = conversations.filter((c) => c.id !== id)
      if (activeId !== id) return
      if (!conversations.length) {
        await create()
        return
      }
      await select(conversations[0].id)
    } catch (cause) { reportError(cause, 'Delete session') }
  }

  const answerPermission = async (decision: 'allow' | 'allow_session' | 'deny') => {
    if (!permission) return
    const id = permission.id
    permission = null
    try { await desktop.answerPermission(id, decision) } catch (cause) { reportError(cause, 'Permission response') }
  }

  const answerQuestion = async (answers: QuestionAnswer[]) => {
    if (!question) return
    const request = question
    question = null
    try { await desktop.answerQuestion(request.id, answers) } catch (cause) { reportError(cause, 'Question response') }
  }

  const saveSettings = async (s: SettingsPayload) => {
    try {
      await desktop.saveSettings(s)
      provider = s.provider
      model = s.model
      smallModel = s.small_model
      modelLabel = catalog.modelName(model, provider) ?? model
      thinking = s.thinking as ReasoningEffort
      autostartEngine = s.autostart_engine
      apiKey = ''
      customUrl = s.custom_url
      clearError()
      toast.success('Settings applied to Crush')
    } catch (cause) { reportError(cause, 'Save settings') }
  }

  const applySelection = () => {
    if (!provider || !model) return
    void saveSettings({ theme: '', autostart_engine: autostartEngine, provider, model, small_model: smallModel, thinking, api_key: '', custom_url: customUrl })
  }

  const setModel = (next: string, label?: string, providerID?: string, type: ModelType = 'large') => {
    if (type === 'small') {
      smallModel = next
      void applySelection()
      return
    }
    model = next
    modelLabel = label ?? catalog.modelName(next, providerID) ?? next
    if (providerID) provider = providerID
    else {
      const match = catalog.models.find((m) => m.id === next)
      if (match) provider = match.providerId
    }
    void applySelection()
  }

  const setThinking = (value: ReasoningEffort) => {
    thinking = value
    void applySelection()
  }

  return {
    get sessions(): SessionSummary[] { return conversations.map(({ id, title, updatedAt, pinned, status }) => ({ id, title, updatedAt, pinned, streaming: status === 'streaming' })) },
    get activeId() { return activeId }, get active() { return activeConversation() }, get input() { return input }, get workspace() { return workspace }, get backendReady() { return backendReady }, get engine() { return engine }, get error() { return error }, get permission() { return permission }, get question() { return question },
    get provider() { return provider }, get model() { return model }, get smallModel() { return smallModel }, get modelLabel() { return modelLabel }, get thinking() { return thinking }, get thinkingLabel() { const opt = REASONING_EFFORT_OPTIONS.find((o) => o.id === thinking); return opt ? `Think: ${opt.short}` : 'Think: Auto' }, get apiKey() { return apiKey }, get customUrl() { return customUrl }, get autostartEngine() { return autostartEngine },
    setInput: (v: string) => (input = v), setModel, setThinking,
    init, destroy, pickWorkspace, create, select, send, cancel, rename, delete: remove, answerPermission, answerQuestion, loadSettings, saveSettings,
  }
}
