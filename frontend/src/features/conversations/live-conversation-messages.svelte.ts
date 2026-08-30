import { desktop, type MessageInfo, type WorkspaceInfo } from '../../platform/desktop'
import { catalog } from './catalog.svelte'
import { ChatMessage, type Conversation, type Message } from './types.svelte'

const NEW_CONVERSATION_TITLE = 'Hội thoại mới'
const SESSION_MEMORY_PREFIX = 'gotack.active-session:'
const DEFAULT_WORKSPACE_LABEL = 'C:\\'
const RUN_STATE_POLL_MS = 1000

let localSeq = 0
export const localId = (prefix: string) => `${prefix}:${Date.now().toString(36)}:${++localSeq}`

export type MessageDeps = {
  conversations: { value: Conversation[] }
  activeId: { value: string }
  input: { value: string }
  workspace: { value: string }
  streamingText: { value: string }
  reportError: (cause: unknown, prefix?: string) => void
  clearError: () => void
  updateConversation: (id: string, fn: (c: Conversation) => Conversation) => void
  rememberSession: (id: string) => void
  applyLoadedSelection: (providerID?: string, modelID?: string) => Promise<void>
  waitForReady: () => Promise<boolean>
  waitForSelection: () => Promise<boolean>
}
type LoadedSelection = { providerID: string; modelID: string }

const latestSelection = (rows: readonly MessageInfo[]): LoadedSelection | undefined => {
  let latest: MessageInfo | undefined
  for (const row of rows) {
    if (!row.provider || !row.model) continue
    if (!latest || row.created_at >= latest.created_at) latest = row
  }
  return latest ? { providerID: latest.provider, modelID: latest.model } : undefined
}


export function createMessageState(deps: MessageDeps) {
  const activeConversation = () => deps.conversations.value.find((item) => item.id === deps.activeId.value)
  const runWatchers = new Map<string, number>()

  const stopRunWatcher = (id: string) => {
    const timer = runWatchers.get(id)
    if (timer !== undefined) window.clearTimeout(timer)
    runWatchers.delete(id)
  }

  const clearRunWatchers = () => {
    for (const id of runWatchers.keys()) stopRunWatcher(id)
  }

  const watchRunCompletion = (id: string) => {
    stopRunWatcher(id)
    const poll = async () => {
      const local = deps.conversations.value.find((item) => item.id === id)
      if (!local || local.status !== 'streaming') {
        runWatchers.delete(id)
        return
      }
      const rows = await desktop.listSessions().catch(() => null)
      const remote = rows?.find((row) => row.id === id)
      if (remote && !remote.is_busy) {
        deps.updateConversation(id, (c) => ({ ...c, status: 'idle' }))
        deps.streamingText.value = ''
        runWatchers.delete(id)
        return
      }
      runWatchers.set(id, window.setTimeout(() => void poll(), RUN_STATE_POLL_MS))
    }
    runWatchers.set(id, window.setTimeout(() => void poll(), RUN_STATE_POLL_MS))
  }

  const memoryKey = () => `${SESSION_MEMORY_PREFIX}${deps.workspace.value}`

  const loadMessages = async (id: string): Promise<LoadedSelection | undefined> => {
    const rows = await desktop.sessionMessages(id)
    const messages: Message[] = rows
      .filter((m) => m.role === 'user' || m.role === 'assistant')
      .map((m) => {
        const inst = new ChatMessage(m.id, m.role as 'user' | 'assistant', m.created_at)
        inst.content = m.text
        return inst
      })
    deps.updateConversation(id, (c) => ({ ...c, messages }))
    return latestSelection(rows)
  }

  const loadSessions = async () => {
    const rows = await desktop.listSessions()
    deps.conversations.value = rows.map((s) => ({
      id: s.id,
      title: s.title || NEW_CONVERSATION_TITLE,
      updatedAt: s.updated_at || Date.now(),
      pinned: false,
      status: s.is_busy ? 'streaming' : 'idle',
      messages: [],
    }))
    if (!deps.conversations.value.length) {
      const created = await desktop.createSession(NEW_CONVERSATION_TITLE)
      deps.conversations.value = [{
        id: created.id,
        title: created.title || NEW_CONVERSATION_TITLE,
        updatedAt: created.updated_at || Date.now(),
        pinned: false,
        status: 'idle',
        messages: [],
      }]
    }
    const remembered = localStorage.getItem(memoryKey())
    const next = deps.conversations.value.some((c) => c.id === remembered) ? remembered! : deps.conversations.value[0].id
    deps.activeId.value = next
    deps.rememberSession(next)
    await desktop.switchSession(next)
    return loadMessages(next)
  }


  const attachWorkspace = async (workspace: WorkspaceInfo) => {
    clearRunWatchers()
    deps.workspace.value = workspace.is_default ? DEFAULT_WORKSPACE_LABEL : workspace.path
    const selection = await loadSessions()
    await catalog.refresh()
    await deps.applyLoadedSelection(selection?.providerID, selection?.modelID)
    deps.clearError()
  }

  const openWorkspacePath = async (path: string) => {
    if (!path) return
    const opened = await desktop.openWorkspace(path)
    await attachWorkspace(opened)
  }

  const ensureWorkspace = async () => {
    const current = await desktop.currentWorkspace().catch(() => null)
    if (current?.path) {
      await attachWorkspace(current)
      return
    }
    const assistant = await desktop.ensureAssistantWorkspace()
    await attachWorkspace(assistant)
  }

  const pickWorkspace = async () => {
    try {
      const path = await desktop.selectWorkspace()
      if (path) await openWorkspacePath(path)
    } catch (cause) { deps.reportError(cause, 'Open workspace') }
  }

  const create = async () => {
    if (!await deps.waitForReady()) return
    await deps.waitForSelection()
    try {
      const s = await desktop.createSession(NEW_CONVERSATION_TITLE)
      const c: Conversation = {
        id: s.id,
        title: s.title || NEW_CONVERSATION_TITLE,
        updatedAt: s.updated_at || Date.now(),
        pinned: false,
        status: 'idle',
        messages: [],
      }
      deps.conversations.value = [c, ...deps.conversations.value]
      deps.activeId.value = c.id
      deps.rememberSession(c.id)
      await desktop.switchSession(c.id)
    } catch (cause) { deps.reportError(cause, 'Create session') }
  }

  const select = async (id: string) => {
    try {
      deps.activeId.value = id
      deps.rememberSession(id)
      deps.input.value = ''
      await desktop.switchSession(id)
      const selection = await loadMessages(id)
      await deps.applyLoadedSelection(selection?.providerID, selection?.modelID)
      deps.clearError()
    } catch (cause) { deps.reportError(cause, 'Switch session') }
  }

  const send = async () => {
    const text = deps.input.value.trim()
    if (!text) return
    if (!await deps.waitForReady()) return
    if (!await deps.waitForSelection()) return
    let current = activeConversation()
    if (!current) {
      deps.reportError('Hội thoại chưa sẵn sàng', 'Send prompt')
      return
    }
    if (current.status === 'streaming') {
      // session:done is edge-triggered. If the UI missed that event during a
      // stream reconnect, the local conversation can stay "streaming" forever
      // even though Crush has already finished the run. Reconcile with the
      // authoritative session state before rejecting the next prompt.
      const rows = await desktop.listSessions().catch(() => null)
      const remote = rows?.find((row) => row.id === current!.id)
      if (!remote || remote.is_busy) return
      deps.updateConversation(current.id, (c) => ({ ...c, status: 'idle' }))
      current = activeConversation()
      if (!current || current.status === 'streaming') return
    }
    deps.input.value = ''
    deps.streamingText.value = ''
    const userMessage = new ChatMessage(localId('user'), 'user')
    userMessage.content = text
    deps.updateConversation(current.id, (c) => ({ ...c, status: 'streaming', messages: [...c.messages, userMessage] }))
    try {
      await desktop.sendPrompt(current.id, text)
      watchRunCompletion(current.id)
    } catch (cause) {
      stopRunWatcher(current.id)
      deps.reportError(cause, 'Send prompt')
      deps.updateConversation(current.id, (c) => ({ ...c, status: 'idle' }))
    }
  }

  const cancel = async () => {
    const current = activeConversation()
    if (!current || current.status !== 'streaming') return
    try { await desktop.cancelPrompt(current.id) } catch (cause) { deps.reportError(cause, 'Cancel prompt') }
  }

  const rename = async (id: string, title: string) => {
    const clean = title.trim()
    if (!clean) return
    try {
      const saved = await desktop.renameSession(id, clean)
      deps.updateConversation(id, (c) => ({ ...c, title: saved.title || clean, updatedAt: saved.updated_at || Date.now() }))
    } catch (cause) { deps.reportError(cause, 'Rename session') }
  }

  const remove = async (id: string) => {
    try {
      stopRunWatcher(id)
      await desktop.deleteSession(id)
      deps.conversations.value = deps.conversations.value.filter((c) => c.id !== id)
      if (deps.activeId.value !== id) return
      if (!deps.conversations.value.length) {
        await create()
        return
      }
      await select(deps.conversations.value[0].id)
    } catch (cause) { deps.reportError(cause, 'Delete session') }
  }

  return {
    loadMessages,
    loadSessions,
    ensureWorkspace,
    pickWorkspace,
    create,
    select,
    send,
    cancel,
    rename,
    remove,
  }
}
