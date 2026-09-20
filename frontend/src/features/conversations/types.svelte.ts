export type MessageRole = 'user' | 'assistant'

export type ChatAttachment = {
  id: string
  fileName: string
  mimeType: string
  size: number
  content: string
  path?: string
}

export class ChatMessage {
  readonly id: string
  readonly role: MessageRole
  content = $state('')
  attachments = $state<ChatAttachment[]>([])
  seq = 0
  kind: 'message' | 'tool' = 'message'
  toolName?: string
  toolFinished?: boolean
  createdAt = Date.now()

  constructor(id: string, role: MessageRole, createdAt?: number) {
    this.id = id
    this.role = role
    if (createdAt) this.createdAt = createdAt
  }
}

export type Message = ChatMessage

export type ConversationStatus = 'idle' | 'streaming'

export type Conversation = {
  id: string
  title: string
  updatedAt: number
  pinned: boolean
  status: ConversationStatus
  messages: Message[]
}

export type SessionSummary = Pick<Conversation, 'id' | 'title' | 'updatedAt' | 'pinned'> & { streaming: boolean }

export type ReasoningEffort = 'none' | 'minimal' | 'low' | 'medium' | 'high' | 'xhigh' | 'max'
