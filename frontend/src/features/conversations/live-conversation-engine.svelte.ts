import {
  desktop,
  events,
  on,
  type EngineInfo,
  type PermissionRequestPayload as Envelope,
  type QuestionRequestEvent,
  type SessionDeltaEvent,
  type SessionDoneEvent,
  type ToolActivityEvent,
} from '../../platform/desktop'
import { applyDelta } from './merge-delta'
import { ChatMessage, type Conversation, type ModelType, type ReasoningEffort } from './types.svelte'
import { catalog } from './catalog.svelte'

const RECONNECT_MAX_MS = 30_000
type SettingsPayload = { theme: string; autostart_engine: boolean; provider: string; model: string; small_model: string; thinking: string; api_key: string; custom_url: string }


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
  smallModel: { value: string }
  thinking: { value: ReasoningEffort }
  apiKey: { value: string }
  customUrl: { value: string }
  autostartEngine: { value: boolean }
  reportError: (cause: unknown, prefix?: string) => void
  clearError: () => void
  updateConversation: (id: string, fn: (c: Conversation) => Conversation) => void
  ensureWorkspace: () => Promise<void>
}

export function createEngineState(deps: EngineDeps) {
  const unsubscribers: Array<() => void> = []

  let reconnectTimer: number | undefined
  let reconnectAttempt = 0
  let destroyed = false
  let attachedTo = ''

  const clearReconnect = () => {
    if (reconnectTimer !== undefined) window.clearTimeout(reconnectTimer)
    reconnectTimer = undefined
  }

  const scheduleReconnect = () => {
    if (destroyed || !deps.backendReady.value || reconnectTimer !== undefined) return
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
      try {
        await deps.ensureWorkspace()
        if (catalog.status !== 'ready') {
          await catalog.refresh()
          deps.applyLoadedSelection()
        }
      } catch (cause) {
        attachedTo = ''
        deps.reportError(cause, 'Restore workspace')
      }
      return
    }
    attachedTo = ''
    catalog.reset()
    if (info.error) deps.error.value = info.error
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
              // Wire seq chain broke (gap, restart, or out-of-order).
              // We rebuilt from the server snapshot, so rendering is
              // intact; surface a soft warning rather than crashing.
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
        deps.streamingText.value = event.text
      }),
      on<ToolActivityEvent>(events.toolActivity, (event) => {
        deps.updateConversation(event.session_id, (c) => {
          const id = `tool:${event.tool_call_id || event.name}`
          let m = c.messages.find((x) => x.id === id)
          let content = ''
          try {
            const raw = typeof event.input === 'string' ? event.input : JSON.stringify(event.input)
            content = raw.length > 180 ? `${raw.slice(0, 180)}…` : raw
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
        deps.streamingText.value = ''
        if (event.error) deps.reportError(event.error, 'Agent run')
      }),
      on<Envelope>(events.permissionRequest, (event) => (deps.permission.value = event)),
      on<QuestionRequestEvent>(events.questionRequest, (event) => (deps.question.value = event)),
    )
  }

  // applyLoadedSelection resolves the stored provider/model against the live
  // catalog once it is available. Unknown stored ids keep their raw value so
  // the truth stays visible instead of silently resetting.
  const applyLoadedSelection = () => {
    if (deps.provider.value) deps.modelLabel.value = catalog.modelName(deps.model.value, deps.provider.value) ?? deps.model.value
  }

  const loadSettings = async () => {
    const s = await desktop.getSettings().catch(() => null)
    if (!s) return
    if (s.provider) deps.provider.value = s.provider
    if (s.model) deps.model.value = s.model
    if (s.small_model) deps.smallModel.value = s.small_model
    if (s.thinking) deps.thinking.value = s.thinking as ReasoningEffort
    deps.autostartEngine.value = s.autostart_engine
    deps.apiKey.value = ''
    deps.customUrl.value = s.custom_url ?? ''
    if (catalog.status === 'ready') applyLoadedSelection()
  }

  const init = async () => {
    destroyed = false
    deps.backendReady.value = await desktop.backendReady().catch(() => false)
    if (!deps.backendReady.value) {
      deps.reportError('Wails backend unavailable')
      return
    }
    subscribe()
    await loadSettings()
    try {
      const status = await desktop.startEngine()
      await handleEngine(status)
    } catch (cause) {
      deps.reportError(cause, 'Start Crush')
      scheduleReconnect()
    }
  }

  const destroy = () => {
    destroyed = true
    clearReconnect()
    unsubscribers.forEach((off) => off())
    unsubscribers.length = 0
  }

  const saveSettings = async (s: SettingsPayload) => {
    try {
      await desktop.saveSettings(s)
      deps.provider.value = s.provider
      deps.model.value = s.model
      deps.smallModel.value = s.small_model
      deps.modelLabel.value = catalog.modelName(deps.model.value, deps.provider.value) ?? deps.model.value
      deps.thinking.value = s.thinking as ReasoningEffort
      deps.autostartEngine.value = s.autostart_engine
      deps.apiKey.value = ''
      deps.customUrl.value = s.custom_url
      deps.clearError()
    } catch (cause) { deps.reportError(cause, 'Save settings') }
  }

  const applySelection = () => {
    if (!deps.provider.value || !deps.model.value) return
    const s: SettingsPayload = {
      theme: '',
      autostart_engine: deps.autostartEngine.value,
      provider: deps.provider.value,
      model: deps.model.value,
      small_model: deps.smallModel.value,
      thinking: deps.thinking.value,
      api_key: '',
      custom_url: deps.customUrl.value,
    }
    void saveSettings(s)
  }

  const setModel = (next: string, label?: string, providerID?: string, type: ModelType = 'large') => {
    if (type === 'small') {
      deps.smallModel.value = next
      void applySelection()
      return
    }
    deps.model.value = next
    deps.modelLabel.value = label ?? catalog.modelName(next, providerID) ?? next
    if (providerID) deps.provider.value = providerID
    else {
      const match = catalog.models.find((m) => m.id === next)
      if (match) deps.provider.value = match.providerId
    }
    void applySelection()
  }

  const setThinking = (value: ReasoningEffort) => {
    deps.thinking.value = value
    void applySelection()
  }

  return {
    init,
    destroy,
    loadSettings,
    saveSettings,
    applyLoadedSelection,
    setModel,
    setThinking,
    scheduleReconnect,
  }
}
