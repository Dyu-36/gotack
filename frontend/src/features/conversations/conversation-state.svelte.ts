import type {
  Conversation,
  Message,
  ModelOption,
  ModelType,
  ProviderOption,
  ReasoningEffort,
  SessionSummary,
} from './types'
import { desktop } from '../../platform/desktop'

const NEW_CONVERSATION_TITLE = 'Hội thoại mới'

export const CRUSH_PROVIDERS: ProviderOption[] = [
  {
    id: 'hyper',
    name: 'Charm Hyper',
    type: 'hyper',
    authType: 'oauth_hyper',
    badge: 'Charm',
    apiEndpoint: 'https://hyper.charm.land/api/v1/fantasy',
    description: 'Nền tảng AI tích hợp của Charm (OAuth Device Code)',
    defaultLargeModelId: 'qwen3.7-plus',
    defaultSmallModelId: 'deepseek-v4-flash-0731',
  },
  {
    id: 'copilot',
    name: 'GitHub Copilot',
    type: 'copilot',
    authType: 'oauth_copilot',
    badge: 'GitHub',
    description: 'GitHub Copilot subscription (OAuth Device Flow hoặc import từ VS Code)',
    defaultLargeModelId: 'claude-3.7-sonnet',
    defaultSmallModelId: 'claude-3.5-sonnet',
  },
  {
    id: 'anthropic',
    name: 'Anthropic',
    type: 'anthropic',
    authType: 'api_key',
    badge: 'Claude',
    apiEndpoint: 'https://api.anthropic.com/v1',
    description: 'Mô hình Claude chính hãng (API Key)',
    defaultLargeModelId: 'claude-3-7-sonnet-20250219',
    defaultSmallModelId: 'claude-3-5-haiku-20241022',
  },
  {
    id: 'openai',
    name: 'OpenAI',
    type: 'openai',
    authType: 'api_key',
    badge: 'GPT',
    apiEndpoint: 'https://api.openai.com/v1',
    description: 'GPT-4o, o3-mini, o1 chính hãng (API Key)',
    defaultLargeModelId: 'gpt-4o',
    defaultSmallModelId: 'gpt-4o-mini',
  },
  {
    id: 'google',
    name: 'Google Gemini',
    type: 'google',
    authType: 'api_key',
    badge: 'Gemini',
    apiEndpoint: 'https://generativelanguage.googleapis.com',
    description: 'Gemini 2.5 Pro & Flash (API Key)',
    defaultLargeModelId: 'gemini-2.5-pro',
    defaultSmallModelId: 'gemini-2.5-flash',
  },
  {
    id: 'deepseek',
    name: 'DeepSeek',
    type: 'openai-compat',
    authType: 'api_key',
    badge: 'DeepSeek',
    apiEndpoint: 'https://api.deepseek.com/v1',
    description: 'DeepSeek R1 & V3 chính hãng (API Key)',
    defaultLargeModelId: 'deepseek-reasoner',
    defaultSmallModelId: 'deepseek-chat',
  },
  {
    id: 'openrouter',
    name: 'OpenRouter',
    type: 'openrouter',
    authType: 'api_key',
    badge: 'Router',
    apiEndpoint: 'https://openrouter.ai/api/v1',
    description: 'Định tuyến đa mô hình (API Key)',
    defaultLargeModelId: 'anthropic/claude-3.7-sonnet',
    defaultSmallModelId: 'openai/gpt-4o-mini',
  },
  {
    id: 'groq',
    name: 'Groq',
    type: 'openai-compat',
    authType: 'api_key',
    badge: 'Fast',
    apiEndpoint: 'https://api.groq.com/openai/v1',
    description: 'Tốc độ siêu nhanh với chip LPU (API Key)',
    defaultLargeModelId: 'deepseek-r1-distill-llama-70b',
    defaultSmallModelId: 'llama-3.3-70b-versatile',
  },
  {
    id: 'mistral',
    name: 'Mistral AI',
    type: 'openai-compat',
    authType: 'api_key',
    badge: 'Mistral',
    apiEndpoint: 'https://api.mistral.ai/v1',
    description: 'Codestral & Mistral Large (API Key)',
    defaultLargeModelId: 'codestral-latest',
    defaultSmallModelId: 'mistral-small-latest',
  },
  {
    id: 'xai',
    name: 'xAI (Grok)',
    type: 'openai-compat',
    authType: 'api_key',
    badge: 'Grok',
    apiEndpoint: 'https://api.x.ai/v1',
    description: 'Grok 2 & Grok Beta (API Key)',
    defaultLargeModelId: 'grok-2-latest',
    defaultSmallModelId: 'grok-beta',
  },
  {
    id: 'ollama',
    name: 'Ollama',
    type: 'openai-compat',
    authType: 'endpoint_local',
    badge: 'Local',
    apiEndpoint: 'http://localhost:11434',
    description: 'Mô hình mã nguồn mở cục bộ trên máy',
    defaultLargeModelId: 'qwen2.5-coder',
    defaultSmallModelId: 'llama3.3',
  },
  {
    id: 'lmstudio',
    name: 'LM Studio',
    type: 'openai-compat',
    authType: 'endpoint_local',
    badge: 'Local',
    apiEndpoint: 'http://localhost:1234',
    description: 'Chạy local LLM qua LM Studio API',
  },
  {
    id: 'bedrock',
    name: 'AWS Bedrock',
    type: 'bedrock',
    authType: 'aws_sso',
    badge: 'AWS',
    description: 'Amazon Bedrock Foundation Models (AWS Credentials)',
    defaultLargeModelId: 'anthropic.claude-3-7-sonnet-20250219-v1:0',
  },
  {
    id: 'vertexai',
    name: 'Google Vertex AI',
    type: 'vertexai',
    authType: 'vertex_gcp',
    badge: 'GCP',
    description: 'Google Cloud Vertex AI Enterprise (GCP Project)',
    defaultLargeModelId: 'claude-3-7-sonnet@20250219',
  },
  {
    id: 'azure',
    name: 'Azure OpenAI',
    type: 'azure',
    authType: 'azure_openai',
    badge: 'Azure',
    description: 'Microsoft Azure OpenAI Service (Endpoint & Version)',
    defaultLargeModelId: 'gpt-4o',
  },
]

export const CRUSH_MODELS: ModelOption[] = [
  // Charm Hyper (Matching provider.json exactly)
  { id: 'qwen3.7-plus', name: 'Qwen 3.7 Plus', providerId: 'hyper', contextWindow: 1000000, canReason: true, defaultReasoningEffort: 'high', tag: 'Large Default', description: 'Model chính của Hyper cho coding' },
  { id: 'deepseek-v4-flash-0731', name: 'DeepSeek V4 Flash 0731', providerId: 'hyper', contextWindow: 1048576, canReason: true, defaultReasoningEffort: 'medium', tag: 'Small Default', description: 'Model nhanh cho task nhỏ' },
  { id: 'deepseek-v4-pro', name: 'DeepSeek V4 Pro', providerId: 'hyper', contextWindow: 1000000, canReason: true, defaultReasoningEffort: 'high', description: 'Khả năng suy luận cao' },
  { id: 'deepseek-v4-flash', name: 'DeepSeek V4 Flash', providerId: 'hyper', contextWindow: 1000000, canReason: true, defaultReasoningEffort: 'high', description: 'Phản hồi nhanh' },
  { id: 'glm-5.2', name: 'GLM-5.2', providerId: 'hyper', contextWindow: 1048576, canReason: true, defaultReasoningEffort: 'high', description: 'Context 1M tokens' },
  { id: 'kimi-k3', name: 'Kimi K3', providerId: 'hyper', contextWindow: 1048576, canReason: true, defaultReasoningEffort: 'max', description: 'Suy luận chuyên sâu' },
  { id: 'kimi-k2.7-code', name: 'Kimi K2.7 Code', providerId: 'hyper', contextWindow: 256000, canReason: false, description: 'Chuyên biệt cho code' },
  { id: 'minimax-m3', name: 'MiniMax M3', providerId: 'hyper', contextWindow: 512000, canReason: true, defaultReasoningEffort: 'medium', description: 'Context 512K tokens' },
  { id: 'qwen3.8-max', name: 'Qwen 3.8 Max', providerId: 'hyper', contextWindow: 1000000, canReason: true, description: 'Model lớn nhất dòng Qwen' },
  { id: 'llama-3.3-70b-instruct', name: 'Llama 3.3 70B Instruct', providerId: 'hyper', contextWindow: 128000, canReason: true, description: 'Mô hình mở Meta' },

  // GitHub Copilot
  { id: 'claude-3.7-sonnet', name: 'Claude 3.7 Sonnet (Copilot)', providerId: 'copilot', canReason: true, tag: 'Mới nhất', description: 'Claude 3.7 qua GitHub Copilot' },
  { id: 'claude-3.7-sonnet-thought', name: 'Claude 3.7 Sonnet Thought', providerId: 'copilot', canReason: true, defaultReasoningEffort: 'high', tag: 'Reasoning', description: 'Claude 3.7 có thinking' },
  { id: 'claude-3.5-sonnet', name: 'Claude 3.5 Sonnet (Copilot)', providerId: 'copilot', tag: 'Coding', description: 'Claude 3.5 qua GitHub Copilot' },
  { id: 'gpt-4o', name: 'GPT-4o (Copilot)', providerId: 'copilot', tag: 'Flagship', description: 'GPT-4o qua GitHub Copilot' },
  { id: 'o3-mini', name: 'o3-mini (Copilot)', providerId: 'copilot', canReason: true, tag: 'Reasoning', description: 'o3-mini qua GitHub Copilot' },
  { id: 'o1', name: 'o1 (Copilot)', providerId: 'copilot', canReason: true, tag: 'Deep Reason', description: 'o1 qua GitHub Copilot' },

  // Anthropic
  { id: 'claude-3-7-sonnet-20250219', name: 'Claude 3.7 Sonnet', providerId: 'anthropic', contextWindow: 200000, canReason: true, defaultReasoningEffort: 'high', tag: 'Large Default', description: 'Hybrid Reasoning & Coding' },
  { id: 'claude-3-5-sonnet-20241022', name: 'Claude 3.5 Sonnet', providerId: 'anthropic', contextWindow: 200000, tag: 'Coding Standard', description: 'Chuẩn mực lập trình' },
  { id: 'claude-3-5-haiku-20241022', name: 'Claude 3.5 Haiku', providerId: 'anthropic', contextWindow: 200000, tag: 'Small Default', description: 'Nhanh, gọn nhẹ' },

  // OpenAI
  { id: 'gpt-4o', name: 'GPT-4o', providerId: 'openai', contextWindow: 128000, tag: 'Large Default', description: 'Mô hình đa phương thức hàng đầu' },
  { id: 'gpt-4o-mini', name: 'GPT-4o Mini', providerId: 'openai', contextWindow: 128000, tag: 'Small Default', description: 'Nhanh và tiết kiệm' },
  { id: 'o3-mini', name: 'o3-mini', providerId: 'openai', contextWindow: 200000, canReason: true, defaultReasoningEffort: 'high', tag: 'Reasoning', description: 'Suy luận logic cao' },
  { id: 'o1', name: 'o1', providerId: 'openai', contextWindow: 200000, canReason: true, defaultReasoningEffort: 'high', tag: 'Deep Reasoning', description: 'Suy luận sâu' },

  // Google Gemini
  { id: 'gemini-2.5-pro', name: 'Gemini 2.5 Pro', providerId: 'google', contextWindow: 1000000, canReason: true, tag: 'Large Default', description: 'Suy luận phức tạp, context 1M' },
  { id: 'gemini-2.5-flash', name: 'Gemini 2.5 Flash', providerId: 'google', contextWindow: 1000000, canReason: true, tag: 'Small Default', description: 'Tốc độ cao, context 1M' },
  { id: 'gemini-2.0-flash', name: 'Gemini 2.0 Flash', providerId: 'google', contextWindow: 1000000, description: 'Đa nhiệm thời gian thực' },

  // DeepSeek
  { id: 'deepseek-reasoner', name: 'DeepSeek R1 (Reasoner)', providerId: 'deepseek', contextWindow: 64000, canReason: true, defaultReasoningEffort: 'high', tag: 'Large Default', description: 'DeepSeek R1 suy luận mở' },
  { id: 'deepseek-chat', name: 'DeepSeek V3 (Chat)', providerId: 'deepseek', contextWindow: 64000, tag: 'Small Default', description: 'DeepSeek V3 tổng quát' },

  // OpenRouter
  { id: 'anthropic/claude-3.7-sonnet', name: 'Claude 3.7 Sonnet (OpenRouter)', providerId: 'openrouter', canReason: true, tag: 'OpenRouter' },
  { id: 'openai/gpt-4o', name: 'GPT-4o (OpenRouter)', providerId: 'openrouter', tag: 'OpenRouter' },
  { id: 'deepseek/deepseek-r1', name: 'DeepSeek R1 (OpenRouter)', providerId: 'openrouter', canReason: true, tag: 'OpenRouter' },

  // Groq
  { id: 'deepseek-r1-distill-llama-70b', name: 'DeepSeek R1 Distill 70B', providerId: 'groq', canReason: true, tag: 'Groq LPU' },
  { id: 'llama-3.3-70b-versatile', name: 'Llama 3.3 70B Versatile', providerId: 'groq', tag: 'Groq LPU' },

  // Mistral
  { id: 'codestral-latest', name: 'Codestral Latest', providerId: 'mistral', tag: 'Code' },
  { id: 'mistral-large-latest', name: 'Mistral Large Latest', providerId: 'mistral', tag: 'Flagship' },

  // xAI
  { id: 'grok-2-latest', name: 'Grok 2 Latest', providerId: 'xai', tag: 'xAI' },

  // Ollama
  { id: 'qwen2.5-coder', name: 'Qwen 2.5 Coder', providerId: 'ollama', tag: 'Local Code' },
  { id: 'deepseek-r1', name: 'DeepSeek R1 (Local)', providerId: 'ollama', canReason: true, tag: 'Local Reason' },
  { id: 'llama3.3', name: 'Llama 3.3', providerId: 'ollama', tag: 'Local' },
]

export const REASONING_EFFORT_OPTIONS: Array<{ id: ReasoningEffort; label: string; short: string }> = [
  { id: 'none', label: 'None (Không suy luận)', short: 'None' },
  { id: 'minimal', label: 'Minimal (Tối thiểu)', short: 'Min' },
  { id: 'low', label: 'Low (Thấp)', short: 'Low' },
  { id: 'medium', label: 'Medium (Vừa)', short: 'Med' },
  { id: 'high', label: 'High (Sâu)', short: 'High' },
  { id: 'xhigh', label: 'Extra High (Rất sâu)', short: 'XHigh' },
  { id: 'max', label: 'Maximum (Tối đa)', short: 'Max' },
]

function seedConversations(): Conversation[] {
  const now = Date.now()
  return [
    {
      id: 'welcome',
      title: 'Bắt đầu với Tack',
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

  let provider = $state('hyper')
  let model = $state('qwen3.7-plus')
  let modelLabel = $state('Qwen 3.7 Plus')
  let smallModel = $state('deepseek-v4-flash-0731')
  let thinking = $state<ReasoningEffort>('high')
  let apiKey = $state('')
  let customUrl = $state('')
  let customModelId = $state('')

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

    const currentModelName = modelLabel || 'Qwen 3.7 Plus'
    const thinkingTag = thinking !== 'none' ? ` [Reasoning: ${thinking}]` : ''
    const response = backendReady
      ? `[${currentModelName}${thinkingTag}] UI đã nối được Wails. Luồng Crush streaming/session API sẽ được gắn vào shell này.`
      : `[${currentModelName}${thinkingTag}] Đây là UI preview của Tack. Đã cấu hình model ${currentModelName} sẵn sàng.`

    queueMicrotask(() => appendMessage(conversationId, { role: 'assistant', content: response }, 'idle'))
  }

  const setModel = (m: string, label?: string, pId?: string, type: ModelType = 'large') => {
    if (type === 'small') {
      smallModel = m
      return
    }
    model = m
    if (label) {
      modelLabel = label
    } else {
      const found = CRUSH_MODELS.find((item) => item.id === m)
      modelLabel = found ? found.name : m
    }
    if (pId) {
      provider = pId
    } else {
      const found = CRUSH_MODELS.find((item) => item.id === m)
      if (found) provider = found.providerId
    }
  }

  const setProvider = (p: string) => {
    provider = p
    const provDef = CRUSH_PROVIDERS.find((item) => item.id === p)
    if (provDef && provDef.defaultLargeModelId) {
      model = provDef.defaultLargeModelId
      const found = CRUSH_MODELS.find((item) => item.id === model)
      modelLabel = found ? found.name : model
    }
    if (provDef && provDef.defaultSmallModelId) {
      smallModel = provDef.defaultSmallModelId
    }
  }

  const setThinking = (t: ReasoningEffort) => {
    thinking = t
  }

  const loadSettings = async () => {
    try {
      if (desktop.available()) {
        const s = await desktop.getSettings()
        if (s.provider) provider = s.provider
        if (s.model) {
          model = s.model
          const found = CRUSH_MODELS.find((item) => item.id === s.model)
          modelLabel = found ? found.name : s.model
        }
        if (s.thinking) thinking = s.thinking as ReasoningEffort
        if (s.api_key) apiKey = s.api_key
        if (s.custom_url) customUrl = s.custom_url
      }
    } catch {
      // preview mode
    }
  }

  const saveSettings = async (s: {
    theme: string
    autostart_engine: boolean
    provider: string
    model: string
    thinking: string
    api_key: string
    custom_url: string
  }) => {
    provider = s.provider
    model = s.model
    const found = CRUSH_MODELS.find((item) => item.id === s.model)
    modelLabel = found ? found.name : s.model
    thinking = s.thinking as ReasoningEffort
    apiKey = s.api_key
    customUrl = s.custom_url

    try {
      if (desktop.available()) {
        await desktop.saveSettings(s)
      }
    } catch {
      // preview fallback
    }
  }

  return {
    get sessions(): SessionSummary[] {
      return conversations.map(({ id, title, updatedAt, pinned, status }) => ({
        id,
        title,
        updatedAt,
        pinned,
        streaming: status === 'streaming',
      }))
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
    get provider() {
      return provider
    },
    get model() {
      return model
    },
    get smallModel() {
      return smallModel
    },
    get modelLabel() {
      return modelLabel
    },
    get thinking() {
      return thinking
    },
    get thinkingLabel() {
      const opt = REASONING_EFFORT_OPTIONS.find((o) => o.id === thinking)
      return opt ? `Think: ${opt.short}` : 'Think: High'
    },
    get apiKey() {
      return apiKey
    },
    get customUrl() {
      return customUrl
    },
    get customModelId() {
      return customModelId
    },
    setInput(value: string) {
      input = value
    },
    setProvider,
    setModel,
    setThinking,
    setApiKey(k: string) { apiKey = k },
    setCustomUrl(u: string) { customUrl = u },
    setCustomModelId(m: string) { customModelId = m },
    loadSettings,
    saveSettings,
    create: createConversation,
    select: selectConversation,
    rename: renameConversation,
    togglePin: togglePinConversation,
    delete: deleteConversation,
    sendPreviewMessage,
  }
}

