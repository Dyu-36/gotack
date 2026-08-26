import type { Conversation, Message, SessionSummary } from './types'

const NEW_CONVERSATION_TITLE = 'Hội thoại mới'

function seedConversations(): Conversation[] {
  const now = Date.now()
  return [
    {
      id: 'welcome',
      title: 'Bắt đầu với Gotack',
      updatedAt: now,
      pinned: true,
      status: 'idle',
      messages: [],
    },
    {
      id: 'workspace',
      title: 'Phân tích workspace',
      updatedAt: now - 3_600_000,
      pinned: false,
      status: 'idle',
      messages: [],
    },
    {
      id: 'crush',
      title: 'Crush coding session',
      updatedAt: now - 86_400_000,
      pinned: false,
      status: 'idle',
      messages: [],
    },
  ]
}

export function createConversationState() {
  let conversations = $state<Conversation[]>(seedConversations())
  let activeId = $state('welcome')
  let input = $state('')

  const activeConversation = () => conversations.find((conversation) => conversation.id === activeId)

  const updateConversation = (id: string, update: (conversation: Conversation) => Conversation) => {
    conversations = conversations.map((conversation) => conversation.id === id ? update(conversation) : conversation)
  }

  const createConversation = () => {
    const conversation: Conversation = {
      id: crypto.randomUUID(),
      title: NEW_CONVERSATION_TITLE,
      updatedAt: Date.now(),
      pinned: false,
      status: 'idle',
      messages: [],
    }
    conversations = [conversation, ...conversations]
    activeId = conversation.id
    input = ''
  }

  const selectConversation = (id: string) => {
    if (!conversations.some((conversation) => conversation.id === id)) return
    activeId = id
    input = ''
  }

  const renameConversation = (id: string, title: string) => {
    const trimmed = title.trim()
    if (!trimmed) return
    updateConversation(id, (conversation) => ({ ...conversation, title: trimmed, updatedAt: Date.now() }))
  }

  const togglePinConversation = (id: string) => {
    updateConversation(id, (conversation) => ({ ...conversation, pinned: !conversation.pinned }))
  }

  const deleteConversation = (id: string) => {
    const remaining = conversations.filter((conversation) => conversation.id !== id)
    conversations = remaining

    if (activeId !== id) return
    if (remaining.length > 0) {
      activeId = remaining[0].id
      input = ''
      return
    }
    createConversation()
  }

  const appendMessage = (id: string, message: Message, status: Conversation['status']) => {
    updateConversation(id, (conversation) => ({
      ...conversation,
      updatedAt: Date.now(),
      status,
      messages: [...conversation.messages, message],
    }))
  }

  const sendPreviewMessage = (backendReady: boolean) => {
    const content = input.trim()
    const conversation = activeConversation()
    if (!content || !conversation || conversation.status === 'streaming') return

    const conversationId = conversation.id
    input = ''
    appendMessage(conversationId, { role: 'user', content }, 'streaming')

    const response = backendReady
      ? 'UI đã nối được Wails. Luồng Crush streaming/session API sẽ được gắn vào shell này ở bước backend.'
      : 'Đây là UI preview của Gotack. Wails/Crush backend chưa được attach trong phiên frontend này.'

    queueMicrotask(() => appendMessage(conversationId, { role: 'assistant', content: response }, 'idle'))
  }

  return {
    get sessions(): SessionSummary[] {
      return conversations.map(({ id, title, updatedAt, pinned, status }) => ({ id, title, updatedAt, pinned, status }))
    },
    get activeId() {
      return activeId
    },
    get active() {
      return activeConversation()
    },
    get input() {
      return input
    },
    setInput(value: string) {
      input = value
    },
    create: createConversation,
    select: selectConversation,
    rename: renameConversation,
    togglePin: togglePinConversation,
    delete: deleteConversation,
    sendPreviewMessage,
  }
}
