export type MessageRole = 'user' | 'assistant'

export type Message = {
  id?: string
  role: MessageRole
  content: string
  kind?: 'message' | 'tool'
  toolName?: string
  toolFinished?: boolean
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

export type SessionSummary = Pick<Conversation, 'id' | 'title' | 'updatedAt' | 'pinned'> & { streaming: boolean }

export type ModelType = 'large' | 'small'
// ReasoningEffort values map to Crush's accepted reasoning_effort inputs;
// none/auto-style values leave the provider default in charge.
export type ReasoningEffort = 'none' | 'low' | 'medium' | 'high' | 'max'
