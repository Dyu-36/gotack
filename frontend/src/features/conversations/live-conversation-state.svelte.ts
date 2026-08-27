import type { Conversation, Message, ModelType, ReasoningEffort, SessionSummary } from './types'
import { CRUSH_MODELS, REASONING_EFFORT_OPTIONS } from './conversation-state.svelte'
import {
  desktop,
  events,
  on,
  type EngineInfo,
  type PermissionRequestEvent,
  type QuestionRequestEvent,
  type SessionDeltaEvent,
  type SessionDoneEvent,
} from '../../platform/desktop'

const NEW_CONVERSATION_TITLE = 'Hội thoại mới'

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

  let provider = $state('hyper')
  let model = $state('qwen3.7-plus')
  let modelLabel = $state('Qwen 3.7 Plus')
  let smallModel = $state('deepseek-v4-flash-0731')
  let thinking = $state<ReasoningEffort>('high')
  let apiKey = $state('')
  let customUrl = $state('')
  let customModelId = $state('')
  let unsubscribers: Array<() => void> = []

  const activeConversation = () => conversations.find((item) => item.id === activeId)
  const updateConversation = (id: string, fn: (c: Conversation) => Conversation) => {
    conversations = conversations.map((c) => c.id === id ? fn(c) : c)
  }

  const loadMessages = async (id: string) => {
    const rows = await desktop.sessionMessages(id)
    const messages: Message[] = rows
      .filter((m) => m.role === 'user' || m.role === 'assistant')
      .map((m) => ({ id: m.id, role: m.role as 'user' | 'assistant', content: m.text }))
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
    activeId = conversations[0].id
    await desktop.switchSession(activeId)
    await loadMessages(activeId)
  }

  const openWorkspacePath = async (path: string) => {
    if (!path) return
    const opened = await desktop.openWorkspace(path)
    workspace = opened.path
    await loadSessions()
  }

  const ensureWorkspace = async () => {
    const current = await desktop.currentWorkspace().catch(() => null)
    if (current?.path) {
      workspace = current.path
      await loadSessions()
      return
    }
    const recent = await desktop.listRecentWorkspaces().catch(() => [])
    if (recent.length) await openWorkspacePath(recent[0])
  }

  const handleEngine = async (info: EngineInfo) => {
    engine = info
    error = info.error ?? ''
    if (info.running) await ensureWorkspace().catch((e) => (error = String(e)))
  }

  const subscribe = () => {
    unsubscribers.forEach((off) => off())
    unsubscribers = [
      on<EngineInfo>(events.engineStatus, (e) => void handleEngine(e)),
      on<SessionDeltaEvent>(events.sessionDelta, (e) => {
        const existing = activeConversation()
        if (!existing || existing.id !== e.session_id) return
        updateConversation(e.session_id, (c) => {
          const index = c.messages.findIndex((m) => m.id === e.message_id)
          const next = [...c.messages]
          if (index >= 0) next[index] = { ...next[index], content: e.text }
          else next.push({ id: e.message_id, role: 'assistant', content: e.text })
          return { ...c, status: 'streaming', updatedAt: Date.now(), messages: next }
        })
      }),
      on<SessionDoneEvent>(events.sessionDone, (e) => {
        updateConversation(e.session_id, (c) => ({ ...c, status: 'idle', updatedAt: Date.now() }))
        if (e.error) error = e.error
      }),
      on<PermissionRequestEvent>(events.permissionRequest, (e) => (permission = e)),
      on<QuestionRequestEvent>(events.questionRequest, (e) => (question = e)),
    ]
  }

  const loadSettings = async () => {
    const s = await desktop.getSettings().catch(() => null)
    if (!s) return
    if (s.provider) provider = s.provider
    if (s.model) {
      model = s.model
      modelLabel = CRUSH_MODELS.find((m) => m.id === model)?.name ?? model
    }
    if (s.thinking) thinking = s.thinking as ReasoningEffort
    apiKey = s.api_key ?? ''
    customUrl = s.custom_url ?? ''
  }

  const init = async () => {
    backendReady = await desktop.backendReady().catch(() => false)
    if (!backendReady) return
    subscribe()
    await loadSettings()
    const status = await desktop.startEngine()
    engine = status
    if (status.running) await ensureWorkspace()
  }

  const destroy = () => {
    unsubscribers.forEach((off) => off())
    unsubscribers = []
  }

  const pickWorkspace = async () => {
    try {
      const path = await desktop.selectWorkspace()
      if (path) await openWorkspacePath(path)
    } catch (e) {
      error = String(e)
    }
  }

  const create = async () => {
    if (!engine?.running || workspace === 'Chọn thư mục...') return
    const s = await desktop.createSession(NEW_CONVERSATION_TITLE)
    const c: Conversation = { id: s.id, title: s.title || NEW_CONVERSATION_TITLE, updatedAt: s.updated_at || Date.now(), pinned: false, status: 'idle', messages: [] }
    conversations = [c, ...conversations]
    activeId = c.id
    await desktop.switchSession(c.id)
  }

  const select = async (id: string) => {
    activeId = id
    input = ''
    await desktop.switchSession(id)
    await loadMessages(id)
  }

  const send = async () => {
    const text = input.trim()
    const current = activeConversation()
    if (!text || !current || current.status === 'streaming' || !backendReady) return
    input = ''
    updateConversation(current.id, (c) => ({ ...c, status: 'streaming', messages: [...c.messages, { role: 'user', content: text }] }))
    try {
      await desktop.sendPrompt(current.id, text)
    } catch (e) {
      error = String(e)
      updateConversation(current.id, (c) => ({ ...c, status: 'idle' }))
    }
  }

  const cancel = async () => {
    if (activeId) await desktop.cancelPrompt(activeId)
  }

  const rename = (id: string, title: string) => updateConversation(id, (c) => ({ ...c, title: title.trim() || c.title }))
  const togglePin = (id: string) => updateConversation(id, (c) => ({ ...c, pinned: !c.pinned }))
  const remove = (id: string) => { conversations = conversations.filter((c) => c.id !== id); if (activeId === id) activeId = conversations[0]?.id ?? '' }

  const answerPermission = async (decision: 'allow' | 'allow_session' | 'deny') => {
    if (!permission) return
    const id = permission.id
    permission = null
    await desktop.answerPermission(id, decision)
  }

  const answerQuestion = async (value: string) => {
    if (!question || !question.questions.length) return
    const request = question
    const q = request.questions[0]
    question = null
    const choice = q.choices?.find((c) => c.id === value)
    await desktop.answerQuestion(request.id, [{ request_id: q.id, selected_ids: choice ? [choice.id] : undefined, fill_in_text: choice ? undefined : value }])
  }

  const saveSettings = async (s: { theme: string; autostart_engine: boolean; provider: string; model: string; thinking: string; api_key: string; custom_url: string }) => {
    provider = s.provider; model = s.model; modelLabel = CRUSH_MODELS.find((m) => m.id === model)?.name ?? model; thinking = s.thinking as ReasoningEffort; apiKey = s.api_key; customUrl = s.custom_url
    await desktop.saveSettings(s)
  }

  const setModel = (m: string, label?: string, pId?: string, type: ModelType = 'large') => {
    if (type === 'small') { smallModel = m; return }
    model = m; modelLabel = label ?? CRUSH_MODELS.find((x) => x.id === m)?.name ?? m
    if (pId) provider = pId
  }

  return {
    get sessions(): SessionSummary[] { return conversations.map(({ id, title, updatedAt, pinned, status }) => ({ id, title, updatedAt, pinned, streaming: status === 'streaming' })) },
    get activeId() { return activeId }, get active() { return activeConversation() }, get input() { return input }, get workspace() { return workspace }, get backendReady() { return backendReady }, get engine() { return engine }, get error() { return error }, get permission() { return permission }, get question() { return question },
    get provider() { return provider }, get model() { return model }, get smallModel() { return smallModel }, get modelLabel() { return modelLabel }, get thinking() { return thinking }, get thinkingLabel() { const opt = REASONING_EFFORT_OPTIONS.find((o) => o.id === thinking); return opt ? `Think: ${opt.short}` : 'Think: High' }, get apiKey() { return apiKey }, get customUrl() { return customUrl }, get customModelId() { return customModelId },
    setInput: (v: string) => (input = v), setModel, setThinking: (v: ReasoningEffort) => (thinking = v), setProvider: (v: string) => (provider = v), setApiKey: (v: string) => (apiKey = v), setCustomUrl: (v: string) => (customUrl = v), setCustomModelId: (v: string) => (customModelId = v),
    init, destroy, pickWorkspace, create, select, send, cancel, rename, togglePin, delete: remove, answerPermission, answerQuestion, loadSettings, saveSettings,
  }
}
