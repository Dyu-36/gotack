import { desktop, type MessageInfo, type PromptFilePick, type WorkspaceInfo } from '../../platform/desktop'
import { catalog } from './catalog.svelte'
import { fileToAttachment, pathToAttachment } from './attachments'
import { ChatMessage, type ChatAttachment, type Conversation, type Message } from './types.svelte'

const NEW_CONVERSATION_TITLE = 'Hội thoại mới'
const SESSION_MEMORY_PREFIX = 'gotack.active-session:'
const DEFAULT_WORKSPACE_LABEL = 'C:\\'

let localSeq = 0
export const localId = (prefix: string) => `${prefix}:${Date.now().toString(36)}:${++localSeq}`

export type MessageDeps = {
  conversations: { value: Conversation[] }
  activeId: { value: string }
  input: { value: string }
  attachments: { value: ChatAttachment[] }
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
  // Guards against a slow history snapshot overwriting a newer selection.
  let loadGeneration = 0

  const memoryKey = () => `${SESSION_MEMORY_PREFIX}${deps.workspace.value}`

  // buildMessages mirrors what the live event stream renders. Crush stores one
  // assistant row per agent step, so steps that only ran tools carry no text;
  // replaying those rows verbatim is what produced the empty assistant bubbles.
  const buildMessages = (rows: readonly MessageInfo[]): Message[] => {
    const out: Message[] = []
    for (const row of rows) {
      if (row.role === 'user') {
        const inst = new ChatMessage(row.id, 'user', row.created_at)
        inst.content = row.text
        inst.attachments = (row.attachments ?? []).map((attachment, index) => ({
          id: `${row.id}:attachment:${index}`,
          fileName: attachment.file_name,
          mimeType: attachment.mime_type,
          size: attachment.size,
          content: attachment.content ?? '',
        }))
        out.push(inst)
        continue
      }
      if (row.role !== 'assistant') continue
      if (row.text.trim()) {
        const inst = new ChatMessage(row.id, 'assistant', row.created_at)
        inst.content = row.text
        out.push(inst)
      }
      for (const call of row.tool_calls ?? []) {
        // Same id scheme as tool:activity so replay and live rows merge.
        const inst = new ChatMessage(`tool:${call.id}`, 'assistant', row.created_at)
        inst.kind = 'tool'
        inst.toolName = call.name
        inst.toolFinished = call.finished
        inst.content = call.input ?? ''
        out.push(inst)
      }
    }
    return out
  }

  const loadMessages = async (id: string): Promise<LoadedSelection | undefined> => {
    // Snapshots are async, so a newer selection must win. Otherwise the reply
    // for the previous conversation lands after the switch and overwrites the
    // history the user is now looking at.
    const generation = ++loadGeneration
    const rows = await desktop.sessionMessages(id)
    if (generation !== loadGeneration) return undefined
    deps.updateConversation(id, (c) => ({ ...c, messages: buildMessages(rows) }))
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
      deps.attachments.value = []
      deps.rememberSession(c.id)
      await desktop.switchSession(c.id)
    } catch (cause) { deps.reportError(cause, 'Create session') }
  }

  const select = async (id: string) => {
    try {
      deps.activeId.value = id
      deps.rememberSession(id)
      deps.input.value = ''
      deps.attachments.value = []
      // Streaming text belongs to the conversation we just left.
      deps.streamingText.value = ''
      await desktop.switchSession(id)
      const selection = await loadMessages(id)
      if (deps.activeId.value !== id) return
      await deps.applyLoadedSelection(selection?.providerID, selection?.modelID)
      deps.clearError()
    } catch (cause) { deps.reportError(cause, 'Switch session') }
  }

  const send = async () => {
    const text = deps.input.value.trim()
    const attachments = deps.attachments.value
    if (!text && attachments.length === 0) return
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
    deps.attachments.value = []
    deps.streamingText.value = ''
    const userMessage = new ChatMessage(localId('user'), 'user')
    userMessage.content = text
    userMessage.attachments = attachments
    deps.updateConversation(current.id, (c) => ({ ...c, status: 'streaming', messages: [...c.messages, userMessage] }))
    try {
      await desktop.sendPrompt(current.id, text, attachments.map((attachment) => ({
        file_name: attachment.fileName,
        mime_type: attachment.mimeType,
        // A path send carries no body: the host reads the file at send time.
        ...(attachment.path ? { path: attachment.path } : { content: attachment.content }),
      })))
    } catch (cause) {
      deps.reportError(cause, 'Send prompt')
      if (!deps.input.value) deps.input.value = text
      if (!deps.attachments.value.length) deps.attachments.value = attachments
      deps.updateConversation(current.id, (c) => ({
        ...c,
        status: 'idle',
        messages: c.messages.filter((message) => message.id !== userMessage.id),
      }))
    }
  }

  // attachPaths turns host-resolved paths (native picker or OS drop) into chips.
  // Nothing is read here, so a multi-megabyte spreadsheet costs no base64 pass
  // through the webview.
  const attachPaths = (picks: readonly PromptFilePick[]) => {
    if (!picks.length) return
    deps.attachments.value = [...deps.attachments.value, ...picks.map((pick) => pathToAttachment(pick))]
    deps.clearError()
  }

  const pickFiles = async () => {
    try {
      attachPaths(await desktop.pickPromptFiles())
    } catch (cause) {
      deps.reportError(cause, 'Đính kèm tệp')
    }
  }

  const attachFiles = async (files: File[]) => {
    for (const file of files) {
      try {
        const attachment = await fileToAttachment(file)
        deps.attachments.value = [...deps.attachments.value, attachment]
        deps.clearError()
      } catch (cause) {
        deps.reportError(cause, 'Đính kèm tệp')
      }
    }
  }

  const removeAttachment = (id: string) => {
    deps.attachments.value = deps.attachments.value.filter((attachment) => attachment.id !== id)
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
    attachFiles,
    attachPaths,
    pickFiles,
    removeAttachment,
    cancel,
    rename,
    remove,
  }
}
