import { desktop } from '../../platform/desktop'
import { catalog } from './catalog.svelte'
import { ChatMessage, type Conversation, type Message } from './types.svelte'

const NEW_CONVERSATION_TITLE = 'Hội thoại mới'
const SESSION_MEMORY_PREFIX = 'gotack.active-session:'
const DEFAULT_WORKSPACE_LABEL = 'Chọn thư mục...'

let localSeq = 0
export const localId = (prefix: string) => `${prefix}:${Date.now().toString(36)}:${++localSeq}`

export type MessageDeps = {
  conversations: { value: Conversation[] }
  activeId: { value: string }
  input: { value: string }
  workspace: { value: string }
  backendReady: { value: boolean }
  streamingText: { value: string }
  reportError: (cause: unknown, prefix?: string) => void
  clearError: () => void
  updateConversation: (id: string, fn: (c: Conversation) => Conversation) => void
  rememberSession: (id: string) => void
  applyLoadedSelection: () => void
}

export function createMessageState(deps: MessageDeps) {
  const activeConversation = () => deps.conversations.value.find((item) => item.id === deps.activeId.value)


  const memoryKey = () => `${SESSION_MEMORY_PREFIX}${deps.workspace.value}`

  const loadMessages = async (id: string) => {
    const rows = await desktop.sessionMessages(id)
    const messages: Message[] = rows
      .filter((m) => m.role === 'user' || m.role === 'assistant')
      .map((m) => {
        const inst = new ChatMessage(m.id, m.role as 'user' | 'assistant')
        inst.content = m.text
        return inst
      })
    deps.updateConversation(id, (c) => ({ ...c, messages }))
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
    await loadMessages(next)
  }


  const attachWorkspace = async (path: string) => {
    deps.workspace.value = path
    await loadSessions()
    await catalog.refresh()
    deps.applyLoadedSelection()
    deps.clearError()
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

  const pickWorkspace = async () => {
    try {
      const path = await desktop.selectWorkspace()
      if (path) await openWorkspacePath(path)
    } catch (cause) { deps.reportError(cause, 'Open workspace') }
  }

  const create = async () => {
    if (!deps.backendReady.value || deps.workspace.value === DEFAULT_WORKSPACE_LABEL) return
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
      await loadMessages(id)
      deps.clearError()
    } catch (cause) { deps.reportError(cause, 'Switch session') }
  }

  const send = async () => {
    const text = deps.input.value.trim()
    const current = activeConversation()
    if (!text || !current || current.status === 'streaming' || !deps.backendReady.value) return
    deps.input.value = ''
    deps.streamingText.value = ''
    const userMessage = new ChatMessage(localId('user'), 'user')
    userMessage.content = text
    deps.updateConversation(current.id, (c) => ({ ...c, status: 'streaming', messages: [...c.messages, userMessage] }))
    try {
      await desktop.sendPrompt(current.id, text)
    } catch (cause) {
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
