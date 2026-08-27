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

export type AuthType = 'oauth_hyper' | 'oauth_copilot' | 'api_key' | 'endpoint_local' | 'aws_sso' | 'vertex_gcp' | 'azure_openai'
export type ProviderOption = { id: string; name: string; type: string; authType: AuthType; badge?: string; apiEndpoint?: string; description?: string; defaultLargeModelId?: string; defaultSmallModelId?: string }
export type ModelOption = { id: string; name: string; providerId: string; contextWindow?: number; defaultMaxTokens?: number; canReason?: boolean; reasoningLevels?: string[]; defaultReasoningEffort?: string; supportsAttachments?: boolean; costIn?: number; costOut?: number; tag?: string; description?: string }
export type ModelType = 'large' | 'small'
export type ReasoningEffort = 'none' | 'minimal' | 'low' | 'medium' | 'high' | 'xhigh' | 'max'
