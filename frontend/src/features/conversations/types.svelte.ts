export type MessageRole = 'user' | 'assistant'

// content holds base64 bytes for a browser upload. path is set instead when the
// host picked the file, received it through an OS drop, or expanded an @[path]
// tag; the host then reads the bytes at send time. Exactly one is populated.
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

// Backwards-compatible alias so callers using the plain `Message` type
// continue to typecheck while the array now holds ChatMessage instances.
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

// ReasoningEffort is filtered against each model's backend catalog metadata;
// models with explicit reasoning levels only expose those supported values.
export type ReasoningEffort = 'none' | 'low' | 'medium' | 'high' | 'xhigh' | 'max'
