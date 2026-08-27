<script lang="ts">
  import { CRUSH_MODELS, CRUSH_PROVIDERS, REASONING_EFFORT_OPTIONS } from '../features/conversations/conversation-state.svelte'
  import type { ReasoningEffort } from '../features/conversations/types'

  type Theme = 'system' | 'light' | 'dark'
  type SettingsPayload = {
    theme: Theme
    autostart_engine: boolean
    provider: string
    model: string
    thinking: string
    api_key: string
    custom_url: string
  }

  type Props = {
    theme: Theme
    provider?: string
    model?: string
    smallModel?: string
    thinking?: string
    apiKey?: string
    customUrl?: string
    autostartEngine?: boolean
    onThemeChange: (theme: Theme) => void
    onSaveSettings?: (settings: SettingsPayload) => void
    onClose: () => void
  }

  let {
    theme,
    provider = 'hyper',
    model = 'qwen3.7-plus',
    thinking = 'high',
    apiKey = '',
    customUrl = '',
    autostartEngine = true,
    onThemeChange,
    onSaveSettings = () => {},
    onClose,
  }: Props = $props()

  let selectedTheme = $state<Theme>('system')
  let selectedProvider = $state('hyper')
  let selectedModel = $state('qwen3.7-plus')
  let selectedThinking = $state<ReasoningEffort>('high')
  let currentApiKey = $state('')
  let currentCustomUrl = $state('')
  let startEngine = $state(true)
  let showApiKey = $state(false)

  $effect(() => {
    selectedTheme = theme
    selectedProvider = provider
    selectedModel = model
    selectedThinking = thinking as ReasoningEffort
    currentApiKey = apiKey
    currentCustomUrl = customUrl
    startEngine = autostartEngine
  })

  let providerInfo = $derived(CRUSH_PROVIDERS.find((item) => item.id === selectedProvider))
  let models = $derived(CRUSH_MODELS.filter((item) => item.providerId === selectedProvider))
  let selectedModelInfo = $derived(CRUSH_MODELS.find((item) => item.providerId === selectedProvider && item.id === selectedModel))
  let supportsAPIKey = $derived(providerInfo?.authType === 'api_key' || providerInfo?.authType === 'azure_openai')
  let supportsEndpoint = $derived(providerInfo?.authType === 'endpoint_local' || providerInfo?.type === 'openai-compat' || selectedProvider === 'azure')
  let upstreamManagedAuth = $derived(providerInfo?.authType === 'oauth_hyper' || providerInfo?.authType === 'oauth_copilot' || providerInfo?.authType === 'aws_sso' || providerInfo?.authType === 'vertex_gcp')

  function chooseProvider(id: string) {
    selectedProvider = id
    const info = CRUSH_PROVIDERS.find((item) => item.id === id)
    const preferred = info?.defaultLargeModelId
    const first = CRUSH_MODELS.find((item) => item.providerId === id)?.id
    selectedModel = preferred && CRUSH_MODELS.some((item) => item.providerId === id && item.id === preferred) ? preferred : (first ?? '')
    const picked = CRUSH_MODELS.find((item) => item.providerId === id && item.id === selectedModel)
    selectedThinking = (picked?.defaultReasoningEffort as ReasoningEffort | undefined) ?? 'none'
    currentApiKey = ''
    currentCustomUrl = info?.apiEndpoint ?? ''
  }

  function save() {
    const payload: SettingsPayload = {
      theme: selectedTheme,
      autostart_engine: startEngine,
      provider: selectedProvider,
      model: selectedModel,
      thinking: selectedThinking,
      api_key: currentApiKey.trim(),
      custom_url: currentCustomUrl.trim(),
    }
    onThemeChange(selectedTheme)
    onSaveSettings(payload)
    // The API key is write-only. Clear the DOM value immediately after handing
    // it to the backend so it does not linger in component state.
    currentApiKey = ''
    onClose()
  }
</script>

<div class="fixed inset-0 z-50 bg-black/35 backdrop-blur-sm flex items-center justify-center p-4" role="presentation">
  <section class="settings-card" role="dialog" aria-modal="true" aria-label="Cài đặt Gotack">
    <header class="px-5 py-4 border-b border-mm-border flex items-center justify-between">
      <div>
        <h2 class="text-base font-semibold text-mm-text">Cài đặt</h2>
        <p class="text-xs text-mm-secondary mt-0.5">Model và credential được áp dụng trực tiếp vào Crush.</p>
      </div>
      <button type="button" class="btn-notion px-2 py-1 text-xs" onclick={onClose}>Đóng</button>
    </header>

    <div class="p-5 overflow-y-auto space-y-6">
      <section class="setting-section">
        <div class="section-title">Giao diện</div>
        <div class="grid grid-cols-3 gap-2">
          {#each ['system', 'light', 'dark'] as value}
            <button type="button" class:active={selectedTheme === value} class="option-btn" onclick={() => (selectedTheme = value as Theme)}>{value === 'system' ? 'Hệ thống' : value === 'light' ? 'Sáng' : 'Tối'}</button>
          {/each}
        </div>
        <label class="toggle-row">
          <span><strong>Tự khởi động Crush</strong><small>Attach engine khi Gotack mở.</small></span>
          <input type="checkbox" bind:checked={startEngine} />
        </label>
      </section>

      <section class="setting-section">
        <div class="section-title">Provider</div>
        <select class="field" value={selectedProvider} onchange={(event) => chooseProvider(event.currentTarget.value)} aria-label="Provider">
          {#each CRUSH_PROVIDERS as item (item.id)}<option value={item.id}>{item.name}</option>{/each}
        </select>
        {#if providerInfo?.description}<p class="hint">{providerInfo.description}</p>{/if}

        {#if upstreamManagedAuth}
          <div class="notice">
            Provider này dùng luồng xác thực do Crush quản lý. Gotack không mô phỏng OAuth hoặc báo “connected” giả; hãy cấu hình credential bằng Crush cho tới khi REST OAuth chính thức được nối vào desktop.
          </div>
        {/if}

        {#if supportsAPIKey}
          <label class="field-label" for="api-key">API key</label>
          <div class="flex gap-2">
            <input id="api-key" class="field flex-1 font-mono" type={showApiKey ? 'text' : 'password'} bind:value={currentApiKey} autocomplete="off" placeholder="Chỉ gửi vào Crush khi Save" />
            <button type="button" class="btn-notion px-3 text-xs" onclick={() => (showApiKey = !showApiKey)}>{showApiKey ? 'Ẩn' : 'Hiện'}</button>
          </div>
          <p class="hint">Gotack không lưu hoặc trả API key về webview. Crush sở hữu persistence credential.</p>
        {/if}

        {#if supportsEndpoint}
          <label class="field-label" for="endpoint">Custom endpoint</label>
          <input id="endpoint" class="field font-mono" bind:value={currentCustomUrl} placeholder={providerInfo?.apiEndpoint ?? 'https://…'} />
          <p class="hint">Ghi vào <code>providers.{selectedProvider}.base_url</code> qua API config của Crush.</p>
        {/if}
      </section>

      <section class="setting-section">
        <div class="section-title">Model</div>
        {#if models.length}
          <select class="field" bind:value={selectedModel} aria-label="Model">
            {#each models as item (item.id)}<option value={item.id}>{item.name}</option>{/each}
          </select>
          {#if selectedModelInfo?.description}<p class="hint">{selectedModelInfo.description}</p>{/if}
        {:else}
          <input class="field font-mono" bind:value={selectedModel} placeholder="Model ID" aria-label="Custom model ID" />
          <p class="hint">Provider không có catalog local; nhập model ID mà Crush/provider chấp nhận.</p>
        {/if}

        <label class="field-label" for="reasoning">Reasoning / Thinking</label>
        <select id="reasoning" class="field" bind:value={selectedThinking}>
          {#each REASONING_EFFORT_OPTIONS as option (option.id)}<option value={option.id}>{option.label}</option>{/each}
        </select>
        <p class="hint">Crush hiện nhận reasoning_effort low/medium/high; Max/XHigh được backend quy về High, Auto/None để provider quyết định.</p>
      </section>
    </div>

    <footer class="px-5 py-3 border-t border-mm-border flex items-center justify-between">
      <span class="text-2xs text-mm-tertiary">Không có test-connection giả hoặc OAuth timer trong UI.</span>
      <div class="flex gap-2">
        <button type="button" class="btn-notion px-3 py-1.5 text-xs" onclick={onClose}>Hủy</button>
        <button type="button" class="px-4 py-1.5 rounded-md bg-mm-accent text-white text-xs font-medium disabled:opacity-40" disabled={!selectedProvider || !selectedModel} onclick={save}>Lưu & áp dụng</button>
      </div>
    </footer>
  </section>
</div>

<style>
  .settings-card { width: min(720px, calc(100vw - 32px)); max-height: min(760px, calc(100vh - 32px)); display: flex; flex-direction: column; overflow: hidden; border: 1px solid var(--mm-border); border-radius: 12px; background: var(--mm-bg); box-shadow: 0 24px 70px rgb(0 0 0 / 24%); }
  .setting-section { display: grid; gap: 10px; }
  .section-title { font-size: 11px; font-weight: 700; color: var(--mm-tertiary); letter-spacing: .08em; text-transform: uppercase; }
  .field-label { margin-top: 3px; font-size: 12px; font-weight: 600; color: var(--mm-secondary); }
  .field { width: 100%; min-height: 36px; padding: 7px 9px; border: 1px solid var(--mm-border); border-radius: 7px; background: var(--mm-panel); color: var(--mm-text); font-size: 12px; outline: none; }
  .field:focus { border-color: var(--mm-accent); }
  .hint { margin: -3px 0 0; font-size: 11px; line-height: 1.5; color: var(--mm-tertiary); }
  .notice { padding: 9px 10px; border: 1px solid var(--mm-border); border-radius: 7px; background: var(--mm-panel); color: var(--mm-secondary); font-size: 11px; line-height: 1.5; }
  .option-btn { padding: 8px; border: 1px solid var(--mm-border); border-radius: 7px; background: var(--mm-panel); color: var(--mm-secondary); font-size: 12px; }
  .option-btn.active { border-color: var(--mm-accent); color: var(--mm-text); box-shadow: 0 0 0 1px var(--mm-accent); }
  .toggle-row { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 9px 0; font-size: 12px; color: var(--mm-text); }
  .toggle-row span { display: grid; gap: 2px; }
  .toggle-row small { color: var(--mm-tertiary); font-size: 10px; }
</style>
