import { toast } from 'svelte-sonner'
import type { EngineInfo, PermissionRequestPayload as Envelope, QuestionRequestEvent } from '../../platform/desktop'
import type { Conversation, ModelType, ReasoningEffort, SessionSummary } from './types.svelte'
import { REASONING_EFFORT_OPTIONS } from './catalog.svelte'
import { createEngineState } from './live-conversation-engine.svelte'
import { createMessageState } from './live-conversation-messages.svelte'
import { createPermissionState } from './live-conversation-permissions.svelte'


const SESSION_MEMORY_PREFIX = 'gotack.active-session:'
const DEFAULT_WORKSPACE_LABEL = 'Chọn thư mục...'

export function createLiveConversationState() {
  // Top-level state owns the shared reactive surface. Sub-states get mutable
  // references into these so writes stay consistent across the whole module.
  let conversations = $state<Conversation[]>([])
  let activeId = $state('')
  let input = $state('')
  let workspace = $state(DEFAULT_WORKSPACE_LABEL)
  let backendReady = $state(false)
  let engine = $state<EngineInfo | null>(null)
  let error = $state('')
  let permission = $state<Envelope | null>(null)
  let question = $state<QuestionRequestEvent | null>(null)
  let streamingText = $state('')
  let provider = $state('')
  let model = $state('')
  let modelLabel = $state('Model mặc định')
  let smallModel = $state('')
  let thinking = $state<ReasoningEffort>('high')
  let apiKey = $state('')
  let customUrl = $state('')
  let autostartEngine = $state(true)

  // Shared helpers consumed by the sub-state factories.
  const errorText = (cause: unknown) => cause instanceof Error ? cause.message : String(cause)
  const reportError = (cause: unknown, prefix = '') => {
    const message = `${prefix}${prefix ? ': ' : ''}${errorText(cause)}`
    error = message
    toast.error(message)
  }
  const clearError = () => { error = '' }
  const updateConversation = (id: string, fn: (c: Conversation) => Conversation) => {
    conversations = conversations.map((c) => c.id === id ? fn(c) : c)
  }
  const rememberSession = (id: string) => {
    if (workspace !== DEFAULT_WORKSPACE_LABEL && id) localStorage.setItem(`${SESSION_MEMORY_PREFIX}${workspace}`, id)
  }

  // Compose sub-states. Order matters: messages first because the engine
  // needs ensureWorkspace from it.
  const messages = createMessageState({
    conversations: { get value() { return conversations }, set value(v) { conversations = v } },
    activeId: { get value() { return activeId }, set value(v) { activeId = v } },

    input: { get value() { return input }, set value(v) { input = v } },
    workspace: { get value() { return workspace }, set value(v) { workspace = v } },
    backendReady: { get value() { return backendReady }, set value(v) { backendReady = v } },
    streamingText: { get value() { return streamingText }, set value(v) { streamingText = v } },
    reportError, clearError, updateConversation, rememberSession,
    applyLoadedSelection: () => engineState.applyLoadedSelection(),
  })

  const engineState = createEngineState({
    conversations: { get value() { return conversations } },
    backendReady: { get value() { return backendReady }, set value(v) { backendReady = v } },
    engine: { get value() { return engine }, set value(v) { engine = v } },
    error: { get value() { return error }, set value(v) { error = v } },
    permission: { get value() { return permission }, set value(v) { permission = v } },
    question: { get value() { return question }, set value(v) { question = v } },
    streamingText: { get value() { return streamingText }, set value(v) { streamingText = v } },
    provider: { get value() { return provider }, set value(v) { provider = v } },
    model: { get value() { return model }, set value(v) { model = v } },
    modelLabel: { get value() { return modelLabel }, set value(v) { modelLabel = v } },
    smallModel: { get value() { return smallModel }, set value(v) { smallModel = v } },
    thinking: { get value() { return thinking }, set value(v) { thinking = v } },
    apiKey: { get value() { return apiKey }, set value(v) { apiKey = v } },
    customUrl: { get value() { return customUrl }, set value(v) { customUrl = v } },
    autostartEngine: { get value() { return autostartEngine }, set value(v) { autostartEngine = v } },
    reportError, clearError, updateConversation,
    ensureWorkspace: () => messages.ensureWorkspace(),
  })

  const permissions = createPermissionState({
    permission: { get value() { return permission }, set value(v) { permission = v } },
    question: { get value() { return question }, set value(v) { question = v } },
    reportError,
  })

  return {
    get sessions(): SessionSummary[] {
      return conversations.map(({ id, title, updatedAt, pinned, status: s }) => ({ id, title, updatedAt, pinned, streaming: s === 'streaming' }))
    },
    get activeId() { return activeId },
    get active() { return conversations.find((item) => item.id === activeId) },
    get input() { return input },
    get workspace() { return workspace },
    get backendReady() { return backendReady },
    get engine() { return engine },
    get error() { return error },
    get permission() { return permission },
    get question() { return question },
    get streamingText() { return streamingText },
    get provider() { return provider },
    get model() { return model },
    get modelLabel() { return modelLabel },
    get smallModel() { return smallModel },
    get thinking() { return thinking },
    get thinkingLabel() {
      const opt = REASONING_EFFORT_OPTIONS.find((o) => o.id === thinking)
      return opt ? `Think: ${opt.short}` : 'Think: Auto'
    },
    get apiKey() { return apiKey },
    get customUrl() { return customUrl },
    get autostartEngine() { return autostartEngine },
    get permissionSecondsLeft() { return permissions.permissionSecondsLeft.value },
    get permissionExpired() { return permissions.permissionExpired.value },

    setInput: (v: string) => { input = v },
    setModel: (next: string, label?: string, providerID?: string, type: ModelType = 'large') => engineState.setModel(next, label, providerID, type),
    setThinking: (value: ReasoningEffort) => engineState.setThinking(value),
    init: () => engineState.init(),
    destroy: () => engineState.destroy(),
    pickWorkspace: () => messages.pickWorkspace(),
    create: () => messages.create(),
    select: (id: string) => messages.select(id),
    send: () => messages.send(),
    cancel: () => messages.cancel(),
    rename: (id: string, title: string) => messages.rename(id, title),
    delete: (id: string) => messages.remove(id),
    answerPermission: (decision: 'allow' | 'allow_session' | 'deny') => permissions.answerPermission(decision),
    answerQuestion: (answers: Array<{ request_id: string; selected_ids?: string[]; fill_in_text?: string; yes?: boolean | null }>) => permissions.answerQuestion(answers),
    loadSettings: () => engineState.loadSettings(),
    saveSettings: (s: { theme: string; autostart_engine: boolean; provider: string; model: string; small_model: string; thinking: string; api_key: string; custom_url: string }) => engineState.saveSettings(s),
  }
}
