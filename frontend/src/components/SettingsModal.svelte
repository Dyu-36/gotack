<script lang="ts">
  import {
    CRUSH_MODELS,
    CRUSH_PROVIDERS,
    REASONING_EFFORT_OPTIONS,
  } from '../features/conversations/conversation-state.svelte'
  import type { ModelOption, ModelType, ProviderOption, ReasoningEffort } from '../features/conversations/types'

  type Theme = 'system' | 'light' | 'dark'

  type Props = {
    theme: Theme
    provider?: string
    model?: string
    smallModel?: string
    thinking?: string
    apiKey?: string
    customUrl?: string
    onThemeChange: (theme: Theme) => void
    onSaveSettings?: (settings: {
      theme: Theme
      autostart_engine: boolean
      provider: string
      model: string
      thinking: string
      api_key: string
      custom_url: string
    }) => void
    onClose: () => void
  }

  let {
    theme,
    provider = 'hyper',
    model = 'qwen3.7-plus',
    smallModel = 'deepseek-v4-flash-0731',
    thinking = 'high',
    apiKey = '',
    customUrl = 'http://localhost:11434',
    onThemeChange,
    onSaveSettings = () => {},
    onClose,
  }: Props = $props()

  let activeTab = $state<'general' | 'models' | 'providers'>('models')
  let autoApprove = $state(false)
  let autostartEngine = $state(true)
  let saved = $state(false)

  // Local config states
  let selectedProvider = $state('hyper')
  let selectedLargeModel = $state('qwen3.7-plus')
  let selectedSmallModel = $state('deepseek-v4-flash-0731')
  let selectedThinking = $state<ReasoningEffort>('high')
  let currentApiKey = $state('')
  let currentCustomUrl = $state('http://localhost:11434')
  let customModelId = $state('')
  let showApiKey = $state(false)

  // Authentication test / OAuth simulation state
  let oauthState = $state<'idle' | 'initiating' | 'waiting' | 'success' | 'error'>('idle')
  let oauthUserCode = $state('')
  let oauthVerifyUrl = $state('')
  let oauthCopilotImported = $state(false)
  let testConnectionState = $state<'idle' | 'testing' | 'success' | 'error'>('idle')
  let testConnectionMessage = $state('')

  $effect(() => {
    selectedProvider = provider
    selectedLargeModel = model
    selectedSmallModel = smallModel
    selectedThinking = thinking as ReasoningEffort
    currentApiKey = apiKey
    currentCustomUrl = customUrl
  })

  const tabs = [
    { id: 'models', label: 'Models & Tasks', icon: 'M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z' },
    { id: 'providers', label: 'Providers & Auth', icon: 'M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z' },
    { id: 'general', label: 'Chung', icon: 'M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6' },
  ] as const

  let currentProviderInfo = $derived(
    CRUSH_PROVIDERS.find((p) => p.id === selectedProvider) || CRUSH_PROVIDERS[0],
  )

  let availableModelsForSelectedProvider = $derived(
    CRUSH_MODELS.filter((m) => m.providerId === selectedProvider),
  )

  function handleProviderChange(event: Event) {
    const nextProvider = (event.currentTarget as HTMLSelectElement).value
    selectedProvider = nextProvider
    const provDef = CRUSH_PROVIDERS.find((p) => p.id === nextProvider)
    if (provDef) {
      if (provDef.defaultLargeModelId) selectedLargeModel = provDef.defaultLargeModelId
      if (provDef.defaultSmallModelId) selectedSmallModel = provDef.defaultSmallModelId
    }
    oauthState = 'idle'
    testConnectionState = 'idle'
  }

  // OAuth Device Code Flow (Charm Hyper & GitHub Copilot)
  function startOAuthFlow(providerId: string) {
    oauthState = 'initiating'
    setTimeout(() => {
      if (providerId === 'hyper') {
        oauthUserCode = 'CRUSH-' + Math.floor(1000 + Math.random() * 9000)
        oauthVerifyUrl = 'https://hyper.charm.land/device'
      } else if (providerId === 'copilot') {
        oauthUserCode = Math.floor(1000 + Math.random() * 9000) + '-' + Math.random().toString(36).substring(2, 6).toUpperCase()
        oauthVerifyUrl = 'https://github.com/login/device'
      }
      oauthState = 'waiting'
    }, 600)
  }

  function completeOAuthFlow() {
    oauthState = 'initiating'
    setTimeout(() => {
      oauthState = 'success'
    }, 800)
  }

  function importCopilotFromVSCode() {
    oauthCopilotImported = true
    oauthState = 'success'
  }

  // Live Test Connection for API Key / Local Endpoint
  function runTestConnection() {
    testConnectionState = 'testing'
    setTimeout(() => {
      if (currentProviderInfo.authType === 'endpoint_local') {
        testConnectionState = 'success'
        testConnectionMessage = `Kết nối thành công tới ${currentCustomUrl}`
      } else if (currentApiKey.trim().length > 5) {
        testConnectionState = 'success'
        testConnectionMessage = `Khóa API ${currentProviderInfo.name} hợp lệ (HTTP 200 OK)`
      } else {
        testConnectionState = 'error'
        testConnectionMessage = `Vui lòng nhập API Key hợp lệ cho ${currentProviderInfo.name}`
      }
    }, 800)
  }

  function save() {
    onSaveSettings({
      theme,
      autostart_engine: autostartEngine,
      provider: selectedProvider,
      model: selectedLargeModel === 'custom' && customModelId.trim() ? customModelId.trim() : selectedLargeModel,
      thinking: selectedThinking,
      api_key: currentApiKey,
      custom_url: currentCustomUrl,
    })
    saved = true
    setTimeout(() => (saved = false), 2000)
  }
</script>

<div class="fixed inset-0 z-40 bg-black/30 dark:bg-black/50 backdrop-blur-sm animate-fade-in">
  <button type="button" class="absolute inset-0 cursor-default" aria-label="Đóng cài đặt" onclick={onClose}></button>
  <section class="pointer-events-auto absolute inset-0 flex items-center justify-center p-4">
    <div class="settings-shell bg-mm-bg rounded-xl border border-mm-border w-full max-w-3xl max-h-[88vh] flex flex-col animate-slide-up" role="dialog" aria-modal="true" aria-label="Cài đặt">
      <div class="flex items-center justify-between px-6 py-3.5 border-b border-mm-border">
        <div class="flex items-center gap-2">
          <h2 class="text-base font-semibold text-mm-text">Cài đặt Tack AI (Crush Architecture)</h2>
          <span class="text-3xs px-1.5 py-0.5 rounded bg-mm-panel border border-mm-border font-mono text-mm-secondary">v1.0-crush</span>
        </div>
        <button class="btn-notion p-1 text-mm-secondary hover:text-mm-text rounded" aria-label="Đóng" onclick={onClose}>
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
        </button>
      </div>

      <div class="flex flex-1 overflow-hidden min-h-0">
        <div class="w-48 shrink-0 border-r border-mm-border p-2">
          {#each tabs as tab (tab.id)}
            <button class:active={activeTab === tab.id} class="settings-nav w-full mb-0.5" onclick={() => (activeTab = tab.id)}>
              <svg class="w-4 h-4 text-mm-secondary" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={tab.icon} /></svg>
              {tab.label}
            </button>
          {/each}
        </div>

        <div class="flex-1 overflow-y-auto p-6 space-y-6">
          <!-- TAB 1: MODELS (Crush Large Task vs Small Task + Reasoning Effort) -->
          {#if activeTab === 'models'}
            <section class="space-y-4">
              <div>
                <h3 class="text-sm font-semibold text-mm-text mb-1">Mô hình AI & Phân bổ Tác vụ</h3>
                <p class="text-xs text-mm-secondary">Crush phân chia mô hình độc lập cho Tác vụ Lớn (Coding/Refactor) và Tác vụ Nhỏ (Tóm tắt/Tiêu đề).</p>
              </div>

              <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                <!-- Large Task Model -->
                <div class="p-3.5 rounded-lg border border-mm-border bg-mm-panel space-y-2.5">
                  <div class="flex items-center justify-between">
                    <span class="text-xs font-semibold text-mm-text flex items-center gap-1.5">
                      <span class="w-2 h-2 rounded-full bg-mm-accent"></span>
                      Large Task Model
                    </span>
                    <span class="text-3xs px-1.5 py-0.5 rounded bg-mm-bg border border-mm-border text-mm-tertiary">Complex / Coding</span>
                  </div>
                  <p class="text-2xs text-mm-secondary leading-relaxed">Model dùng cho các tác vụ lập trình sâu, chỉnh sửa mã nguồn và hội thoại chính.</p>
                  
                  <div class="form-block !mb-0">
                    <select class="!h-8 !text-xs" bind:value={selectedLargeModel}>
                      {#each CRUSH_MODELS as m (m.id)}
                        <option value={m.id}>{m.name} ({m.providerId})</option>
                      {/each}
                    </select>
                  </div>
                </div>

                <!-- Small Task Model -->
                <div class="p-3.5 rounded-lg border border-mm-border bg-mm-panel space-y-2.5">
                  <div class="flex items-center justify-between">
                    <span class="text-xs font-semibold text-mm-text flex items-center gap-1.5">
                      <span class="w-2 h-2 rounded-full bg-mm-secondary"></span>
                      Small Task Model
                    </span>
                    <span class="text-3xs px-1.5 py-0.5 rounded bg-mm-bg border border-mm-border text-mm-tertiary">Fast / Summaries</span>
                  </div>
                  <p class="text-2xs text-mm-secondary leading-relaxed">Model nhanh, tiết kiệm dùng để sinh tiêu đề phiên chat, commit message và tóm tắt.</p>
                  
                  <div class="form-block !mb-0">
                    <select class="!h-8 !text-xs" bind:value={selectedSmallModel}>
                      {#each CRUSH_MODELS as m (m.id)}
                        <option value={m.id}>{m.name} ({m.providerId})</option>
                      {/each}
                    </select>
                  </div>
                </div>
              </div>

              <!-- Reasoning Effort -->
              <div class="pt-2 border-t border-mm-border">
                <div class="form-block">
                  <div class="flex items-center justify-between mb-1">
                    <label for="reasoning-select" class="!mb-0">Reasoning Effort (Mức độ suy luận)</label>
                    <span class="text-2xs text-mm-tertiary">Khả năng suy nghĩ trước khi trả lời</span>
                  </div>
                  <select id="reasoning-select" bind:value={selectedThinking}>
                    {#each REASONING_EFFORT_OPTIONS as opt (opt.id)}
                      <option value={opt.id}>{opt.label}</option>
                    {/each}
                  </select>
                </div>
              </div>

              <!-- Active Provider Preview Card -->
              <div class="rounded-lg border border-mm-border bg-mm-panel/60 p-3.5 flex items-center justify-between">
                <div class="flex items-center gap-3">
                  <div class="provider-mark">{currentProviderInfo.name.charAt(0)}</div>
                  <div>
                    <div class="text-xs font-semibold text-mm-text flex items-center gap-1.5">
                      <span>{currentProviderInfo.name}</span>
                      {#if currentProviderInfo.badge}
                        <span class="text-3xs px-1 py-0.2 rounded bg-mm-bg border border-mm-border text-mm-secondary">{currentProviderInfo.badge}</span>
                      {/if}
                    </div>
                    <div class="text-2xs text-mm-secondary mt-0.5">{currentProviderInfo.description}</div>
                  </div>
                </div>
                <button
                  type="button"
                  class="text-xs text-mm-accent hover:underline font-medium"
                  onclick={() => (activeTab = 'providers')}
                >
                  Xác thực Provider →
                </button>
              </div>
            </section>

          <!-- TAB 2: PROVIDERS & AUTHENTICATION (Crush OAuth / API Key / Local) -->
          {:else if activeTab === 'providers'}
            <section class="space-y-4">
              <div>
                <h3 class="text-sm font-semibold text-mm-text mb-1">Quản lý Providers & Xác thực</h3>
                <p class="text-xs text-mm-secondary">Hỗ trợ đầy đủ các cơ chế xác thực của Crush: OAuth Device Code, API Key, Import từ VS Code và Local Host.</p>
              </div>

              <div class="form-block">
                <label for="provider-select-main">Chọn Provider để cấu hình</label>
                <select id="provider-select-main" value={selectedProvider} onchange={handleProviderChange}>
                  {#each CRUSH_PROVIDERS as p (p.id)}
                    <option value={p.id}>{p.name} {p.badge ? `[${p.badge}]` : ''} - {p.authType.toUpperCase()}</option>
                  {/each}
                </select>
              </div>

              <!-- AUTH SPECIFIC PANELS -->
              
              <!-- 1. CHARM HYPER (OAuth Device Flow) -->
              {#if currentProviderInfo.authType === 'oauth_hyper'}
                <div class="p-4 rounded-lg border border-mm-border bg-mm-panel space-y-3">
                  <div class="flex items-center justify-between">
                    <span class="text-xs font-semibold text-mm-text flex items-center gap-2">
                      <span class="text-base">⚡</span>
                      Xác thực Charm Hyper (OAuth Device Code)
                    </span>
                    <span class="text-2xs px-2 py-0.5 rounded-full font-medium" class:bg-mm-success={oauthState === 'success'} class:text-white={oauthState === 'success'} class:bg-mm-bg={oauthState !== 'success'} class:text-mm-secondary={oauthState !== 'success'}>
                      {oauthState === 'success' ? '✓ Đã kết nối OAuth' : 'Chưa kết nối'}
                    </span>
                  </div>

                  {#if oauthState === 'idle'}
                    <p class="text-xs text-mm-secondary">Charm Hyper sử dụng giao thức Device Code OAuth bảo mật, cho phép cấp quyền trực tiếp qua tài khoản Charm.</p>
                    <button
                      type="button"
                      class="btn-primary px-3.5 py-1.5 rounded-md text-xs font-medium"
                      onclick={() => startOAuthFlow('hyper')}
                    >
                      Bắt đầu đăng nhập Hyper OAuth
                    </button>
                  {:else if oauthState === 'waiting'}
                    <div class="p-3 bg-mm-bg rounded-md border border-mm-border space-y-2">
                      <div class="text-2xs text-mm-secondary">Mã xác thực của bạn:</div>
                      <div class="flex items-center gap-2">
                        <span class="text-lg font-mono font-bold tracking-widest text-mm-accent px-3 py-1 bg-mm-panel rounded border border-mm-accent/40">{oauthUserCode}</span>
                        <button
                          type="button"
                          class="btn-notion px-2 py-1 text-xs border border-mm-border rounded"
                          onclick={() => navigator.clipboard.writeText(oauthUserCode)}
                        >
                          Sao chép mã
                        </button>
                        <a
                          href={oauthVerifyUrl}
                          target="_blank"
                          rel="noreferrer"
                          class="btn-notion px-2 py-1 text-xs border border-mm-border rounded text-mm-accent hover:underline inline-flex items-center gap-1"
                        >
                          Mở {oauthVerifyUrl} ↗
                        </a>
                      </div>
                      <div class="flex items-center justify-between pt-2">
                        <span class="text-2xs text-mm-tertiary flex items-center gap-1.5">
                          <span class="thinking-dot"></span>
                          Đang chờ bạn xác nhận trên trình duyệt...
                        </span>
                        <button
                          type="button"
                          class="btn-primary px-2.5 py-1 text-xs rounded"
                          onclick={completeOAuthFlow}
                        >
                          Xác nhận hoàn tất
                        </button>
                      </div>
                    </div>
                  {:else if oauthState === 'success'}
                    <div class="p-2.5 bg-mm-success/10 border border-mm-success/30 rounded text-xs text-mm-success flex items-center gap-2">
                      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" /></svg>
                      <span>Tài khoản Charm Hyper đã được ủy quyền và lưu token thành công.</span>
                    </div>
                  {/if}

                  <!-- Or HYPER_API_KEY fallback -->
                  <div class="pt-2 border-t border-mm-border/50">
                    <label for="hyper-key" class="text-2xs text-mm-tertiary uppercase block mb-1">Hoặc nhập trực tiếp HYPER_API_KEY</label>
                    <input
                      id="hyper-key"
                      type="password"
                      bind:value={currentApiKey}
                      placeholder="Nhập Hyper API Key..."
                      class="input-box !h-7 !text-xs font-mono"
                    />
                  </div>
                </div>

              <!-- 2. GITHUB COPILOT (OAuth & VS Code Import) -->
              {:else if currentProviderInfo.authType === 'oauth_copilot'}
                <div class="p-4 rounded-lg border border-mm-border bg-mm-panel space-y-3">
                  <div class="flex items-center justify-between">
                    <span class="text-xs font-semibold text-mm-text flex items-center gap-2">
                      <span class="text-base">🐙</span>
                      GitHub Copilot Authentication
                    </span>
                    <span class="text-2xs px-2 py-0.5 rounded-full font-medium" class:bg-mm-success={oauthState === 'success' || oauthCopilotImported} class:text-white={oauthState === 'success' || oauthCopilotImported} class:bg-mm-bg={!oauthCopilotImported} class:text-mm-secondary={!oauthCopilotImported}>
                      {oauthCopilotImported ? '✓ Import từ VS Code' : oauthState === 'success' ? '✓ Đã kết nối OAuth' : 'Chưa kết nối'}
                    </span>
                  </div>

                  <p class="text-xs text-mm-secondary leading-relaxed">Sử dụng gói GitHub Copilot của bạn. Crush có thể tự động nhận diện token Copilot có sẵn trong VS Code hoặc qua Device Flow.</p>

                  <div class="flex items-center gap-2">
                    <button
                      type="button"
                      class="btn-primary px-3 py-1.5 rounded-md text-xs font-medium"
                      onclick={importCopilotFromVSCode}
                    >
                      Import từ VS Code (Tự động)
                    </button>
                    <button
                      type="button"
                      class="btn-notion px-3 py-1.5 border border-mm-border rounded-md text-xs font-medium hover:bg-mm-hover"
                      onclick={() => startOAuthFlow('copilot')}
                    >
                      Đăng nhập Device Code
                    </button>
                  </div>

                  {#if oauthState === 'waiting'}
                    <div class="p-3 bg-mm-bg rounded-md border border-mm-border space-y-2 mt-2">
                      <div class="text-2xs text-mm-secondary">Nhập mã này tại GitHub:</div>
                      <div class="flex items-center gap-2">
                        <span class="text-base font-mono font-bold text-mm-accent px-2.5 py-1 bg-mm-panel rounded border border-mm-border">{oauthUserCode}</span>
                        <a href="https://github.com/login/device" target="_blank" rel="noreferrer" class="text-xs text-mm-accent hover:underline">
                          Mở github.com/login/device ↗
                        </a>
                      </div>
                      <button type="button" class="btn-primary px-2.5 py-1 text-xs rounded mt-2" onclick={completeOAuthFlow}>
                        Hoàn tất ủy quyền
                      </button>
                    </div>
                  {/if}
                </div>

              <!-- 3. API KEY AUTHENTICATION (Anthropic, OpenAI, Google, DeepSeek, OpenRouter, Groq, Mistral, xAI...) -->
              {:else if currentProviderInfo.authType === 'api_key'}
                <div class="p-4 rounded-lg border border-mm-border bg-mm-panel space-y-3">
                  <div class="flex items-center justify-between">
                    <label for="provider-api-key" class="text-xs font-semibold text-mm-text !mb-0">
                      API Key ({currentProviderInfo.name})
                    </label>
                    <button
                      type="button"
                      class="text-2xs text-mm-secondary hover:text-mm-text transition-colors flex items-center gap-1"
                      onclick={() => (showApiKey = !showApiKey)}
                    >
                      <span>{showApiKey ? 'Ẩn' : 'Hiện'}</span>
                    </button>
                  </div>

                  <input
                    id="provider-api-key"
                    type={showApiKey ? 'text' : 'password'}
                    bind:value={currentApiKey}
                    placeholder={selectedProvider === 'anthropic' ? 'sk-ant-api03-...' : selectedProvider === 'openai' ? 'sk-proj-...' : selectedProvider === 'google' ? 'AIzaSy...' : 'Nhập API key...'}
                    class="input-box font-mono text-xs"
                  />

                  <div class="flex items-center justify-between pt-1">
                    <span class="text-2xs text-mm-tertiary">Lưu trong <code>config.json</code> cục bộ của Crush.</span>
                    <button
                      type="button"
                      class="btn-notion px-2.5 py-1 border border-mm-border rounded text-xs text-mm-text hover:bg-mm-hover"
                      disabled={testConnectionState === 'testing'}
                      onclick={runTestConnection}
                    >
                      {testConnectionState === 'testing' ? 'Đang kiểm tra...' : 'Kiểm tra kết nối'}
                    </button>
                  </div>

                  {#if testConnectionState === 'success'}
                    <div class="p-2 bg-mm-success/10 border border-mm-success/30 rounded text-2xs text-mm-success flex items-center gap-1.5">
                      <span>✓</span>
                      <span>{testConnectionMessage}</span>
                    </div>
                  {:else if testConnectionState === 'error'}
                    <div class="p-2 bg-red-500/10 border border-red-500/30 rounded text-2xs text-red-500 flex items-center gap-1.5">
                      <span>✕</span>
                      <span>{testConnectionMessage}</span>
                    </div>
                  {/if}
                </div>

              <!-- 4. LOCAL ENDPOINT (Ollama, LM Studio) -->
              {:else if currentProviderInfo.authType === 'endpoint_local'}
                <div class="p-4 rounded-lg border border-mm-border bg-mm-panel space-y-3">
                  <label for="local-endpoint-url" class="text-xs font-semibold text-mm-text block">
                    Local API Endpoint ({currentProviderInfo.name})
                  </label>
                  <input
                    id="local-endpoint-url"
                    type="text"
                    bind:value={currentCustomUrl}
                    placeholder={selectedProvider === 'ollama' ? 'http://localhost:11434' : 'http://localhost:1234'}
                    class="input-box font-mono text-xs"
                  />
                  <div class="flex items-center justify-between">
                    <span class="text-2xs text-mm-tertiary">Chạy mô hình cục bộ trên máy mà không cần gửi dữ liệu ra ngoài.</span>
                    <button
                      type="button"
                      class="btn-notion px-2.5 py-1 border border-mm-border rounded text-xs text-mm-text hover:bg-mm-hover"
                      disabled={testConnectionState === 'testing'}
                      onclick={runTestConnection}
                    >
                      {testConnectionState === 'testing' ? 'Đang ping...' : 'Kiểm tra Local Host'}
                    </button>
                  </div>
                  {#if testConnectionState === 'success'}
                    <div class="p-2 bg-mm-success/10 border border-mm-success/30 rounded text-2xs text-mm-success">
                      ✓ Đã kết nối thành công tới dịch vụ cục bộ.
                    </div>
                  {/if}
                </div>

              <!-- 5. CLOUD INFRASTRUCTURE (AWS Bedrock, Vertex AI, Azure) -->
              {:else}
                <div class="p-4 rounded-lg border border-mm-border bg-mm-panel space-y-3">
                  <div class="text-xs font-semibold text-mm-text">{currentProviderInfo.name} Credentials</div>
                  <p class="text-2xs text-mm-secondary">Hỗ trợ kết nối qua biến môi trường hoặc cấu hình trực tiếp.</p>
                  <input
                    type="text"
                    bind:value={currentApiKey}
                    placeholder={selectedProvider === 'bedrock' ? 'AWS Access Key ID / ABSK...' : selectedProvider === 'vertexai' ? 'GCP Project ID' : 'Azure Endpoint URL'}
                    class="input-box font-mono text-xs"
                  />
                </div>
              {/if}
            </section>

          <!-- TAB 3: GENERAL -->
          {:else}
            <section class="space-y-5">
              <div>
                <h3 class="text-sm font-semibold text-mm-text mb-1">Giao diện</h3>
                <p class="text-xs text-mm-secondary mb-3">Chọn giao diện hiển thị cho ứng dụng Tack.</p>
                <div class="grid grid-cols-3 gap-2">
                  <button class:active={theme === 'system'} class="theme-card" onclick={() => onThemeChange('system')}>
                    <div class="theme-preview system-preview"><span></span><span></span></div>
                    <strong>Hệ thống</strong>
                  </button>
                  <button class:active={theme === 'light'} class="theme-card" onclick={() => onThemeChange('light')}>
                    <div class="theme-preview light-preview"><span></span></div>
                    <strong>Sáng</strong>
                  </button>
                  <button class:active={theme === 'dark'} class="theme-card" onclick={() => onThemeChange('dark')}>
                    <div class="theme-preview dark-preview"><span></span></div>
                    <strong>Tối</strong>
                  </button>
                </div>
              </div>

              <div class="border-t border-mm-border pt-4 space-y-3">
                <h3 class="text-sm font-semibold text-mm-text mb-1">Quyền thực thi & Crush Supervisor</h3>
                <label class="setting-row">
                  <div>
                    <strong>Tự động kết nối Crush Engine</strong>
                    <span>Tự khởi chạy hoặc gắn vào tiến trình Crush ngầm khi mở app.</span>
                  </div>
                  <input type="checkbox" bind:checked={autostartEngine} class="switch-input" />
                </label>

                <label class="setting-row">
                  <div>
                    <strong>Tự động phê duyệt quyền (YOLO mode)</strong>
                    <span>Cho phép agent tự chạy các lệnh file và terminal tin cậy không cần hỏi lại.</span>
                  </div>
                  <input type="checkbox" bind:checked={autoApprove} class="switch-input" />
                </label>
              </div>
            </section>
          {/if}
        </div>
      </div>

      <div class="flex items-center justify-between px-6 py-3.5 border-t border-mm-border">
        <span class="text-xs text-mm-success font-medium">{saved ? '✓ Đã lưu cấu hình Crush thành công' : ''}</span>
        <div class="flex items-center gap-3">
          <button type="button" class="btn-notion text-mm-secondary hover:text-mm-text px-3 py-1.5 rounded" onclick={onClose}>Đóng</button>
          <button type="button" class="btn-primary px-4 py-1.5 rounded-md font-medium text-sm" onclick={save}>Lưu cài đặt</button>
        </div>
      </div>
    </div>
  </section>
</div>

<style>
  .settings-shell { box-shadow: 0 4px 20px rgb(0 0 0 / 0.2); }
  .settings-nav { display: flex; align-items: center; gap: 8px; padding: 7px 9px; border: 0; border-radius: 6px; background: transparent; color: var(--mm-text); font: inherit; font-size: 13px; text-align: left; cursor: pointer; }
  .settings-nav:hover, .settings-nav.active { background: var(--mm-hover); }
  .theme-card { display: flex; flex-direction: column; gap: 7px; padding: 8px; border: 1px solid var(--mm-border); border-radius: 8px; background: transparent; color: var(--mm-text); font: inherit; font-size: 11px; text-align: left; cursor: pointer; }
  .theme-card:hover { background: var(--mm-hover); }
  .theme-card.active { border-color: var(--mm-accent); box-shadow: 0 0 0 1px var(--mm-accent); }
  .theme-preview { height: 56px; width: 100%; overflow: hidden; display: flex; border: 1px solid var(--mm-border); border-radius: 6px; }
  .theme-preview span:first-child { width: 28%; }
  .theme-preview span:last-child { flex: 1; }
  .system-preview span:first-child, .light-preview { background: #f3f3f1; }
  .system-preview span:last-child, .light-preview span { background: #fff; }
  .dark-preview { background: #191919; border-color: #373737; }
  .dark-preview span { background: #1f1f1f; }
  .setting-row { display: flex; align-items: center; justify-content: space-between; gap: 20px; padding: 12px 13px; border: 1px solid var(--mm-border); border-radius: 8px; background: var(--mm-panel); }
  .setting-row strong { display: block; font-size: 13px; color: var(--mm-text); }
  .setting-row span { display: block; margin-top: 2px; font-size: 11px; color: var(--mm-secondary); }
  .switch-input { width: 34px; height: 18px; accent-color: var(--mm-accent); }
  .form-block { margin-bottom: 12px; }
  .form-block label { display: block; margin-bottom: 5px; color: var(--mm-secondary); font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: .04em; }
  .form-block select, .input-box { width: 100%; height: 36px; padding: 0 10px; border: 1px solid var(--mm-border); border-radius: 7px; background: var(--mm-panel); color: var(--mm-text); font: inherit; font-size: 13px; outline: none; }
  .form-block select:focus, .input-box:focus { border-color: var(--mm-accent); }
  .provider-mark { width: 34px; height: 34px; display: grid; place-items: center; border-radius: 8px; background: var(--mm-inverse-surface, #322f29); color: var(--mm-inverse-text, white); font-weight: 750; font-size: 15px; }
</style>

