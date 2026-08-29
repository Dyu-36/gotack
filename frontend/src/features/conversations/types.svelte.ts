export type MessageRole = 'user' | 'assistant'

export class ChatMessage {
  readonly id: string
  readonly role: MessageRole
  content = $state('')
  seq = 0
  kind: 'message' | 'tool' = 'message'
  toolName?: string
  toolFinished?: boolean

  constructor(id: string, role: MessageRole) {
    this.id = id
    this.role = role
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

export type ModelType = 'large' | 'small'
// ReasoningEffort values map to Crush's accepted reasoning_effort inputs;
// none/auto-style values leave the provider default in charge.
export type ReasoningEffort = 'none' | 'low' | 'medium' | 'high' | 'max'
