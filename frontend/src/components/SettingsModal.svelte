<script lang="ts">
  import { toast } from 'svelte-sonner'
  import { catalog, REASONING_EFFORT_OPTIONS } from '../features/conversations/catalog.svelte'
  import { desktop, type ZaloConfigUpdate, type ZaloStatusInfo } from '../platform/desktop'
  import type { ReasoningEffort } from '../features/conversations/types'

  type Theme = 'system' | 'light' | 'dark'
  type SettingsPayload = {
    theme: Theme
    autostart_engine: boolean
    provider: string
    model: string
    small_model: string
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
    provider = '',
    model = '',
    smallModel = '',
    thinking = 'high',
    apiKey = '',
    customUrl = '',
    autostartEngine = true,
    onThemeChange,
    onSaveSettings = () => {},
    onClose,
  }: Props = $props()

  let selectedTheme = $state<Theme>('system')
  let selectedProvider = $state('')
  let selectedModel = $state('')
  let selectedSmallModel = $state('')
  let selectedThinking = $state<ReasoningEffort>('high')
  let currentApiKey = $state('')
  let currentCustomUrl = $state('')
  let startEngine = $state(true)
  let showApiKey = $state(false)

  let zaloEnabled = $state(false)
  let zaloToken = $state('')
  let zaloChatsInput = $state('')
  let zaloHasToken = $state(false)
  let zaloStatus = $state<ZaloStatusInfo | null>(null)
  let zaloSaving = $state(false)

  $effect(() => {
    selectedTheme = theme
    selectedProvider = provider
    selectedModel = model
    selectedSmallModel = smallModel
    selectedThinking = thinking as ReasoningEffort
    currentApiKey = apiKey
    currentCustomUrl = customUrl
    startEngine = autostartEngine
    if (catalog.status === 'idle') void catalog.refresh()
    void loadZalo()
  })

  async function loadZalo() {
    try {
      const config = await desktop.getZaloConfig()
      zaloEnabled = config.enabled
      zaloHasToken = config.has_token
      zaloChatsInput = config.allowed_chats.join(', ')
      zaloStatus = await desktop.zaloStatus()
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : String(cause))
    }
  }

  let providerInfo = $derived(catalog.provider(selectedProvider))
  let models = $derived(providerInfo?.models ?? [])

  function chooseProvider(id: string) {
    selectedProvider = id
    const info = catalog.provider(id)
    const preferred = info?.default_large_model_id
    selectedModel = preferred && info?.models.some((item) => item.id === preferred) ? preferred : (info?.models[0]?.id ?? '')
    const preferredSmall = info?.default_small_model_id
    selectedSmallModel = preferredSmall && info?.models.some((item) => item.id === preferredSmall) ? preferredSmall : ''
    selectedThinking = (info?.models.find((item) => item.id === selectedModel)?.default_reasoning_effort as ReasoningEffort | undefined) ?? 'none'
    currentApiKey = ''
    currentCustomUrl = info?.api_endpoint ?? ''
  }

  function save() {
    const payload: SettingsPayload = {
      theme: selectedTheme,
      autostart_engine: startEngine,
      provider: selectedProvider,
      model: selectedModel,
      small_model: selectedSmallModel,
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

  async function saveZalo() {
    zaloSaving = true
    try {
      const allowed = zaloChatsInput.split(',').map((id) => id.trim()).filter(Boolean)
      const update: ZaloConfigUpdate = { enabled: zaloEnabled, allowed_chats: allowed }
      if (zaloToken.trim()) update.token = zaloToken.trim()
      zaloStatus = await desktop.saveZaloConfig(update)
      zaloHasToken = zaloHasToken || zaloToken.trim() !== ''
      zaloToken = ''
      toast.success(zaloStatus.bot_name ? `Zalo đã nối: ${zaloStatus.bot_name}` : 'Đã lưu cấu hình Zalo')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : String(cause))
    } finally {
      zaloSaving = false
    }
  }
</script>

<div class="fixed inset-0 z-50 bg-black/35 backdrop-blur-sm flex items-center justify-center p-4" role="presentation">
  <div class="settings-card" role="dialog" aria-modal="true" aria-label="Cài đặt Gotack">
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
        {#if catalog.status === 'ready'}
          <select class="field" value={selectedProvider} onchange={(event) => chooseProvider(event.currentTarget.value)} aria-label="Provider">
            <option value="" disabled>Chọn provider</option>
            {#each catalog.providers as item (item.id)}<option value={item.id}>{item.name}</option>{/each}
          </select>
          {#if selectedProvider}
            <label class="field-label" for="endpoint">Custom endpoint (tùy chọn)</label>
            <input id="endpoint" class="field font-mono" bind:value={currentCustomUrl} placeholder={providerInfo?.api_endpoint ?? 'https://…'} />
            <p class="hint">Ghi vào <code>providers.{selectedProvider}.base_url</code> qua API config của Crush.</p>
          {/if}
        {:else if catalog.status === 'loading'}
          <p class="hint">Đang tải danh sách provider từ Crush...</p>
        {:else if catalog.status === 'error'}
          <p class="hint">Không tải được provider catalog: {catalog.error}. Hãy mở một workspace rồi thử lại.</p>
        {/if}
      </section>

      <section class="setting-section">
        <div class="section-title">Model & Credential</div>
        {#if selectedProvider}
          <label class="field-label" for="api-key">API key</label>
          <div class="flex gap-2">
            <input id="api-key" class="field flex-1 font-mono" type={showApiKey ? 'text' : 'password'} bind:value={currentApiKey} autocomplete="off" placeholder="Chỉ gửi vào Crush khi Save" />
            <button type="button" class="btn-notion px-3 text-xs" onclick={() => (showApiKey = !showApiKey)}>{showApiKey ? 'Ẩn' : 'Hiện'}</button>
          </div>
          <p class="hint">Gotack không lưu hoặc trả API key về webview. Crush sở hữu persistence credential.</p>

          <label class="field-label" for="model">Model</label>
          {#if models.length}
            <select id="model" class="field" bind:value={selectedModel} aria-label="Model">
              {#each models as item (item.id)}<option value={item.id}>{item.name}</option>{/each}
            </select>
            {#if selectedModel}
              {@const info = models.find((item) => item.id === selectedModel)}
              {#if info}
                <p class="hint">{info.context_window ? `${Math.round(info.context_window / 1000)}K context · ` : ''}${info.cost_per_1m_in ?? 0}/${info.cost_per_1m_out ?? 0} per 1M tokens</p>
              {/if}
            {/if}
          {:else}
            <input id="model" class="field font-mono" bind:value={selectedModel} placeholder="Model ID" aria-label="Custom model ID" />
            <p class="hint">Provider này không có catalog model; nhập model ID mà provider chấp nhận.</p>
          {/if}

          <label class="field-label" for="small-model">Model cho tác vụ nhỏ</label>
          {#if models.length}
            <select id="small-model" class="field" bind:value={selectedSmallModel} aria-label="Small task model">
              <option value="">Mặc định của provider</option>
              {#each models as item (item.id)}<option value={item.id}>{item.name}</option>{/each}
            </select>
            <p class="hint">Dùng cho các tác vụ đơn giản, tiêu tốn ít token hơn model chính.</p>
          {:else}
            <input id="small-model" class="field font-mono" bind:value={selectedSmallModel} placeholder="Small model ID (tùy chọn)" aria-label="Small task model ID" />
          {/if}

          <label class="field-label" for="reasoning">Reasoning / Thinking</label>
          <select id="reasoning" class="field" bind:value={selectedThinking}>
            {#each REASONING_EFFORT_OPTIONS as option (option.id)}<option value={option.id}>{option.label}</option>{/each}
          </select>
          <p class="hint">Crush nhận reasoning_effort low/medium/high; Max được quy về High, None để provider tự quyết định.</p>
        {:else}
          <p class="hint">Chọn một provider để cấu hình model và credential.</p>
        {/if}
      </section>

      <section class="setting-section">
        <div class="section-title">Zalo</div>
        <label class="toggle-row">
          <span><strong>Kết nối Zalo Bot</strong><small>Nhận yêu cầu và trả kết quả qua Zalo khi bạn vắng mặt.</small></span>
          <input type="checkbox" bind:checked={zaloEnabled} />
        </label>

        <label class="field-label" for="zalo-token">Bot token {zaloHasToken ? '(đã lưu, bỏ trống để giữ)' : ''}</label>
        <input id="zalo-token" class="field font-mono" type="password" bind:value={zaloToken} autocomplete="off" placeholder={zaloHasToken ? '••••••••' : 'Token từ Zalo Bot Platform'} />
        <p class="hint">Tạo bot và lấy token tại <code>bot.zaloplatforms.com</code>. Token chỉ lưu trên máy bạn.</p>

        <label class="field-label" for="zalo-chats">Chat ID được phép</label>
        <input id="zalo-chats" class="field font-mono" bind:value={zaloChatsInput} placeholder="VD: 1234567890, 987654321" />
        <p class="hint">Chỉ các chat trong danh sách được phục vụ. Nhắn tin cho bot trước, ID sẽ hiện ở trạng thái bên dưới để bạn sao chép.</p>

        {#if zaloStatus}
          <div class="notice">
            {#if zaloStatus.bot_name}<div><strong>Bot:</strong> {zaloStatus.bot_name}{zaloStatus.running ? ' · đang chạy' : ''}</div>{/if}
            {#if zaloStatus.last_chat_id}<div><strong>Tin nhắn gần nhất:</strong> {zaloStatus.last_sender} · <code>{zaloStatus.last_chat_id}</code> · “{zaloStatus.last_text}”</div>{/if}
            {#if zaloStatus.last_error}<div class="text-red-500"><strong>Lỗi:</strong> {zaloStatus.last_error}</div>{/if}
            {#if !zaloStatus.bot_name && !zaloStatus.last_error}<div>Chưa kết nối. Lưu token rồi bật kết nối.</div>{/if}
          </div>
        {/if}

        <div class="flex justify-end">
          <button type="button" class="px-3 py-1.5 rounded-md bg-mm-accent text-white text-xs font-medium disabled:opacity-40" disabled={zaloSaving || (zaloEnabled && !zaloHasToken && !zaloToken.trim())} onclick={saveZalo}>
            {zaloSaving ? 'Đang lưu...' : 'Lưu & kết nối Zalo'}
          </button>
        </div>
      </section>
    </div>

    <footer class="px-5 py-3 border-t border-mm-border flex items-center justify-between">
      <span class="text-2xs text-mm-tertiary">Cấu hình được áp dụng trực tiếp qua API của Crush.</span>
      <div class="flex gap-2">
        <button type="button" class="btn-notion px-3 py-1.5 text-xs" onclick={onClose}>Hủy</button>
        <button type="button" class="px-4 py-1.5 rounded-md bg-mm-accent text-white text-xs font-medium disabled:opacity-40" disabled={!selectedProvider || !selectedModel} onclick={save}>Lưu & áp dụng</button>
      </div>
    </footer>
  </div>
</div>

<style>
  .settings-card { width: min(720px, calc(100vw - 32px)); max-height: min(760px, calc(100vh - 32px)); display: flex; flex-direction: column; overflow: hidden; border: 1px solid var(--mm-border); border-radius: 12px; background: var(--mm-bg); box-shadow: 0 24px 70px rgb(0 0 0 / 24%); }
  .setting-section { display: grid; gap: 10px; }
  .section-title { font-size: 11px; font-weight: 700; color: var(--mm-tertiary); letter-spacing: .08em; text-transform: uppercase; }
  .field-label { margin-top: 3px; font-size: 12px; font-weight: 600; color: var(--mm-secondary); }
  .field { width: 100%; min-height: 36px; padding: 7px 9px; border: 1px solid var(--mm-border); border-radius: 7px; background: var(--mm-panel); color: var(--mm-text); font-size: 12px; outline: none; }
  .field:focus { border-color: var(--mm-accent); }
  .hint { margin: -3px 0 0; font-size: 11px; line-height: 1.5; color: var(--mm-tertiary); }
  .notice { padding: 9px 10px; border: 1px solid var(--mm-border); border-radius: 7px; background: var(--mm-panel); color: var(--mm-secondary); font-size: 11px; line-height: 1.5; display: grid; gap: 4px; }
  .option-btn { padding: 8px; border: 1px solid var(--mm-border); border-radius: 7px; background: var(--mm-panel); color: var(--mm-secondary); font-size: 12px; }
  .option-btn.active { border-color: var(--mm-accent); color: var(--mm-text); box-shadow: 0 0 0 1px var(--mm-accent); }
  .toggle-row { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 9px 0; font-size: 12px; color: var(--mm-text); }
  .toggle-row span { display: grid; gap: 2px; }
  .toggle-row small { color: var(--mm-tertiary); font-size: 10px; }
</style>
