export type MessageRole = 'user' | 'assistant'

export type Message = {
  role: MessageRole
  content: string
}

export type ConversationStatus = 'idle' | 'streaming'

export type Conversation = {
  id: string
  title: string
  updatedAt: number
  pinned: boolean
  status: ConversationStatus
  messages: Message[]
}

export type SessionSummary = Pick<Conversation, 'id' | 'title' | 'updatedAt' | 'pinned'> & {
  streaming: boolean
}
