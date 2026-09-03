import {
  desktop,
  events,
  on,
  type EngineInfo,
  type PermissionRequestPayload as Envelope,
  type PromptFilePick,
  type QuestionRequestEvent,
  type QuestionResolvedEvent,
  type SessionDeltaEvent,
  type SessionDoneEvent,
  type ToolActivityEvent,
} from '../../platform/desktop'
import { setAttachmentLimit } from './attachments'
import { applyDelta } from './merge-delta'
import { ChatMessage, type Conversation, type ReasoningEffort } from './types.svelte'
import { catalog } from './catalog.svelte'

const RECONNECT_MAX_MS = 30_000
type SettingsPayload = { theme: string; provider: string; credential_provider?: string; provider_only?: boolean; model: string; thinking: string; api_key: string; custom_url: string }

export type EngineDeps = {
  conversations: { value: Conversation[] }
  backendReady: { value: boolean }
  engine: { value: EngineInfo | null }
  error: { value: string }
  permission: { value: Envelope | null }
  question: { value: QuestionRequestEvent | null }
  streamingText: { value: string }
  provider: { value: string }
  model: { value: string }
  modelLabel: { value: string }
  thinking: { value: ReasoningEffort }
  apiKey: { value: string }
  customUrl: { value: string }

  activeId: { value: string }
  reportError: (cause: unknown, prefix?: string) => void
  clearError: () => void
  updateConversation: (id: string, fn: (c: Conversation) => Conversation) => void
  ensureWorkspace: () => Promise<void>

  reloadMessages: (id: string) => Promise<unknown>

  attachPaths: (picks: PromptFilePick[]) => void
}

export function createEngineState(deps: EngineDeps) {
  const unsubscribers: Array<() => void> = []

  let reconnectTimer: number | undefined
  let reconnectAttempt = 0
  let destroyed = false
  let attachedTo = ''
  let hostReady = false
  let selectionApply: Promise<boolean> = Promise.resolve(true)
  let readyWaiters: Array<(ready: boolean) => void> = []

  const settleReadyWaiters = (ready: boolean) => {
    const waiters = readyWaiters
    readyWaiters = []
    waiters.forEach((resolve) => resolve(ready))
  }

  const waitForReady = () => {
    if (deps.backendReady.value) return Promise.resolve(true)
    if (destroyed) return Promise.resolve(false)
    return new Promise<boolean>((resolve) => readyWaiters.push(resolve))
  }

  const clearReconnect = () => {
    if (reconnectTimer !== undefined) window.clearTimeout(reconnectTimer)
    reconnectTimer = undefined
  }

  const scheduleReconnect = () => {
    if (destroyed || !hostReady || reconnectTimer !== undefined) return
    const delay = Math.min(RECONNECT_MAX_MS, 750 * (2 ** reconnectAttempt))
    reconnectAttempt += 1
    reconnectTimer = window.setTimeout(() => {
      reconnectTimer = undefined
      void desktop.reconnectEngine().catch((cause) => {
        deps.reportError(cause, 'Reconnect failed')
        scheduleReconnect()
      })
    }, delay)
  }

  const engineFingerprint = (info: EngineInfo) => `${info.endpoint}|${info.version}`

  const handleEngine = async (info: EngineInfo) => {
    deps.engine.value = info
    if (info.running === true) {
      const fp = engineFingerprint(info)
      if (attachedTo === fp) return
      attachedTo = fp
      deps.backendReady.value = false
      try {
        await deps.ensureWorkspace()
        if (catalog.status !== 'ready') {
          await catalog.refresh()
          await applyLoadedSelection()
        }
        if (attachedTo !== fp) return
        deps.backendReady.value = true
        settleReadyWaiters(true)
      } catch (cause) {
        attachedTo = ''
        settleReadyWaiters(false)
        deps.reportError(cause, 'Restore workspace')
      }
      return
    }
    attachedTo = ''
    deps.backendReady.value = false
    catalog.reset()
    if (info.status !== 'starting') settleReadyWaiters(false)
    if (info.error) deps.reportError(info.error)
    if (info.status === 'error') scheduleReconnect()
  }

  const subscribe = () => {
    unsubscribers.forEach((off) => off())
    unsubscribers.length = 0
    unsubscribers.push(
      on<EngineInfo>(events.engineStatus, (info) => { void handleEngine(info) }),
      on<SessionDeltaEvent>(events.sessionDelta, (event) => {
        deps.updateConversation(event.session_id, (c) => {
          let m = c.messages.find((x) => x.id === event.message_id)
          const prev = m ? { text: m.content, seq: m.seq } : null
          const result = applyDelta(prev, event.append, event.seq, event.text)
          if (m) {
            m.content = result.text
            m.seq = result.seq
            if (result.kind === 'resync') {

              console.warn(
                `session:delta resync for ${event.message_id} ` +
                  `(prev seq=${prev?.seq ?? 'null'}, got ${event.seq})`,
              )
            }
          } else {
            m = new ChatMessage(event.message_id, 'assistant')
            m.content = result.text
            m.seq = result.seq
            if (result.kind === 'resync') {
              console.warn(
                `session:delta resync (new message) for ${event.message_id} ` +
                  `(prev seq=null, got ${event.seq})`,
              )
            }
            return { ...c, status: 'streaming', updatedAt: Date.now(), messages: [...c.messages, m] }
          }
          c.status = 'streaming'
          c.updatedAt = Date.now()
          return c
        })

        if (event.session_id === deps.activeId.value) deps.streamingText.value = event.text
      }),
      on<ToolActivityEvent>(events.toolActivity, (event) => {
        deps.updateConversation(event.session_id, (c) => {
          const id = `tool:${event.tool_call_id || event.name}`
          let m = c.messages.find((x) => x.id === id)
          let content = ''
          try {
            const raw = typeof event.input === 'string' ? event.input : JSON.stringify(event.input)
            content = raw.length > 4000 ? `${raw.slice(0, 4000)}…` : raw
          } catch { content = '' }
          if (m) {
            m.content = content
            m.kind = 'tool'
            m.toolName = event.name
            m.toolFinished = event.finished
          } else {
            m = new ChatMessage(id, 'assistant')
            m.kind = 'tool'
            m.toolName = event.name
            m.toolFinished = event.finished
            m.content = content
            return { ...c, messages: [...c.messages, m] }
          }
          return c
        })
      }),
      on<SessionDoneEvent>(events.sessionDone, (event) => {
        deps.updateConversation(event.session_id, (c) => ({ ...c, status: 'idle', updatedAt: Date.now() }))
        if (deps.question.value?.session_id === event.session_id) deps.question.value = null
        if (event.session_id === deps.activeId.value) {
          deps.streamingText.value = ''

          void deps.reloadMessages(event.session_id)
        }
        if (event.error) deps.reportError(event.error, 'Agent run')
      }),
      on<Envelope>(events.permissionRequest, (event) => (deps.permission.value = event)),
      on<QuestionRequestEvent>(events.questionRequest, (event) => (deps.question.value = event)),
      on<QuestionResolvedEvent>(events.questionResolved, (event) => {
        if (deps.question.value?.id === event.batch_id) deps.question.value = null
      }),

      on<PromptFilePick[]>(events.promptFiles, (picks) => deps.attachPaths(picks ?? [])),
    )
  }

  const reasoningEfforts = new Set<ReasoningEffort>(['none', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max'])

  const normalizeThinkingForModel = (value: ReasoningEffort, providerID = deps.provider.value, modelID = deps.model.value): ReasoningEffort => {
    const selected = catalog.models.find((m) => m.id === modelID && (!providerID || m.providerId === providerID))
    if (!selected) return value
    if (!selected.can_reason) return 'none'

    const levels = (selected.reasoning_levels ?? []).filter((level): level is ReasoningEffort => reasoningEfforts.has(level as ReasoningEffort))
    if (levels.length === 0) return value === 'none' ? 'none' : 'high'
    if (levels.includes(value)) return value

    const preferred = selected.default_reasoning_effort as ReasoningEffort | undefined
    if (preferred && levels.includes(preferred)) return preferred
    return levels[0]
  }

  const applyLoadedSelection = async (providerID?: string, modelID?: string) => {
    const restored = Boolean(providerID && modelID)
    if (providerID && modelID) {
      deps.provider.value = providerID
      deps.model.value = modelID
    }
    if (deps.provider.value) deps.modelLabel.value = catalog.modelName(deps.model.value, deps.provider.value) ?? deps.model.value
    const normalized = normalizeThinkingForModel(deps.thinking.value)
    const thinkingChanged = normalized !== deps.thinking.value
    if (thinkingChanged) deps.thinking.value = normalized
    if (restored || thinkingChanged) await queueSelection()
  }

  const loadSettings = async () => {
    const s = await desktop.getSettings().catch(() => null)
    if (!s) return
    if (s.provider) deps.provider.value = s.provider
    if (s.model) deps.model.value = s.model
    if (s.thinking) deps.thinking.value = s.thinking as ReasoningEffort
    deps.apiKey.value = ''
    deps.customUrl.value = s.custom_url ?? ''
    if (catalog.status === 'ready') void applyLoadedSelection()
  }

  const init = async () => {
    destroyed = false
    deps.backendReady.value = false
    hostReady = await desktop.backendReady().catch(() => false)
    if (!hostReady) {
      settleReadyWaiters(false)
      deps.reportError('Wails backend unavailable')
      return
    }
    subscribe()

    const limits = await desktop.attachmentLimits().catch(() => null)
    if (limits?.max_bytes) setAttachmentLimit(limits.max_bytes)
    await loadSettings()
    try {
      let status = await desktop.startEngine()
      await handleEngine(status)

      for (let attempt = 0; !status.running && status.status !== 'error' && attempt < 24 && !destroyed; attempt += 1) {
        await new Promise((resolve) => window.setTimeout(resolve, 250))
        status = await desktop.engineStatus()
        await handleEngine(status)
      }
    } catch (cause) {
      settleReadyWaiters(false)
      deps.reportError(cause, 'Start Tack')
      scheduleReconnect()
    }
  }

  const destroy = () => {
    destroyed = true
    hostReady = false
    deps.backendReady.value = false
    settleReadyWaiters(false)
    clearReconnect()
    unsubscribers.forEach((off) => off())
    unsubscribers.length = 0
  }

  const saveSettings = async (s: SettingsPayload, refreshCatalog = true) => {
    try {
      await desktop.saveSettings(s)

      if (refreshCatalog) await catalog.refresh()
      deps.provider.value = s.provider
      deps.model.value = s.model
      deps.modelLabel.value = catalog.modelName(deps.model.value, deps.provider.value) ?? deps.model.value
      deps.thinking.value = s.thinking as ReasoningEffort
      deps.apiKey.value = ''
      if (!s.credential_provider || s.credential_provider === s.provider) deps.customUrl.value = s.custom_url
      deps.clearError()
      return true
    } catch (cause) {
      deps.reportError(cause, 'Save settings')
      return false
    }
  }

  const queueSelection = () => {
    if (!deps.provider.value || !deps.model.value) return selectionApply
    const s: SettingsPayload = {
      theme: '',
      provider: deps.provider.value,
      model: deps.model.value,
      thinking: deps.thinking.value,
      api_key: '',
      custom_url: deps.customUrl.value,
    }
    selectionApply = selectionApply.then(
      () => saveSettings(s, false),
      () => saveSettings(s, false),
    )
    return selectionApply
  }

  const setModel = (next: string, label?: string, providerID?: string) => {
    deps.model.value = next
    deps.modelLabel.value = label ?? catalog.modelName(next, providerID) ?? next
    if (providerID) {
      deps.provider.value = providerID

      const selectedProvider = catalog.provider(providerID)
      deps.customUrl.value = providerID === 'codex' || selectedProvider?.credential_kind === 'oauth'
        ? ''
        : (selectedProvider?.api_endpoint ?? '')
    } else {
      const match = catalog.models.find((m) => m.id === next)
      if (match) deps.provider.value = match.providerId
    }
    deps.thinking.value = normalizeThinkingForModel(deps.thinking.value, deps.provider.value, next)
    void queueSelection()
  }

  const setThinking = (value: ReasoningEffort) => {
    deps.thinking.value = normalizeThinkingForModel(value)
    void queueSelection()
  }

  return {
    init,
    destroy,
    loadSettings,
    saveSettings,
    applyLoadedSelection,
    setModel,
    setThinking,
    waitForReady,
    waitForSelection: () => selectionApply,
    scheduleReconnect,
  }
}
