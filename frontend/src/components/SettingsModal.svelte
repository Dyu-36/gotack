<script lang="ts">
  import { toast } from 'svelte-sonner'
  import { catalog } from '../features/conversations/catalog.svelte'
  import { desktop, type ZaloConfigUpdate, type ZaloStatusInfo } from '../platform/desktop'

  type Theme = 'system' | 'light' | 'dark'
  type Tab = 'providers' | 'zalo' | 'appearance'

  type SettingsPayload = {
    theme: Theme
    autostart_engine: boolean
    provider: string
    credential_provider?: string
    provider_only?: boolean
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
    thinking?: string
    customUrl?: string
    onThemeChange: (theme: Theme) => void
    onSaveSettings?: (settings: SettingsPayload) => void
    onClose: () => void
  }

  let {
    theme,
    provider = '',
    model = '',
    thinking = 'high',
    customUrl = '',
    onThemeChange,
    onSaveSettings = () => {},
    onClose,
  }: Props = $props()

  let activeTab = $state<Tab>('providers')
  let selectedTheme = $state<Theme>('system')
  let selectedProvider = $state('')
  let currentApiKey = $state('')
  let currentCustomUrl = $state('')
  let showApiKey = $state(false)
  let revealedProviderKeys = $state<Record<string, string>>({})
  let revealingProvider = $state('')
  let deletingProvider = $state('')

  let zaloEnabled = $state(false)
  let zaloToken = $state('')
  let zaloHasToken = $state(false)
  let zaloStatus = $state<ZaloStatusInfo | null>(null)
  let zaloSaving = $state(false)
  let zaloBusy = $state(false)
  let zaloPairingCode = $state('')
  let zaloPairedChats = $state<string[]>([])

  $effect(() => {
    selectedTheme = theme
    selectedProvider = provider
    currentApiKey = ''
    currentCustomUrl = customUrl
    if (catalog.status === 'idle') void catalog.refresh()
    void loadZalo()
  })

  async function loadZalo() {
    try {
      const config = await desktop.getZaloConfig()
      zaloEnabled = config.enabled
      zaloHasToken = config.has_token
      zaloPairingCode = config.pairing_code
      // Older hosts may serialize an empty Go slice as null. Keep the tab
      // renderable while the desktop boundary is upgraded to always return [].
      zaloPairedChats = config.paired_chats ?? []
      zaloStatus = await desktop.zaloStatus()
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : String(cause))
    }
  }

  let providerInfo = $derived(catalog.provider(selectedProvider))
  let configuredProviders = $derived(catalog.configuredProviders)

  function chooseProvider(id: string) {
    selectedProvider = id
    currentApiKey = ''
    currentCustomUrl = id === provider ? customUrl : (catalog.provider(id)?.api_endpoint ?? '')
  }

  async function toggleProviderKey(providerID: string) {
    if (revealedProviderKeys[providerID] !== undefined) {
      const next = { ...revealedProviderKeys }
      delete next[providerID]
      revealedProviderKeys = next
      return
    }
    revealingProvider = providerID
    try {
      const key = await desktop.revealProviderAPIKey(providerID)
      revealedProviderKeys = { ...revealedProviderKeys, [providerID]: key }
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : String(cause))
    } finally {
      revealingProvider = ''
    }
  }

  async function deleteConfiguredProvider(providerID: string, name: string) {
    if (!window.confirm(`Xóa cấu hình provider ${name}? Provider sẽ bị tắt trong Tack và credential đã lưu sẽ bị xóa.`)) return
    deletingProvider = providerID
    try {
      await desktop.deleteProvider(providerID)
      const next = { ...revealedProviderKeys }
      delete next[providerID]
      revealedProviderKeys = next
      if (selectedProvider === providerID) {
        selectedProvider = ''
        currentApiKey = ''
        currentCustomUrl = ''
      }
      await catalog.refresh()
      toast.success(`Đã xóa provider ${name}`)
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : String(cause))
    } finally {
      deletingProvider = ''
    }
  }

  function save() {
    const payload: SettingsPayload = {
      theme: selectedTheme,
      autostart_engine: true,
      provider,
      credential_provider: selectedProvider || undefined,
      provider_only: true,
      model,
      small_model: model,
      thinking,
      api_key: currentApiKey.trim(),
      custom_url: selectedProvider ? currentCustomUrl.trim() : '',
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
      const update: ZaloConfigUpdate = { enabled: zaloEnabled }
      if (zaloToken.trim()) update.token = zaloToken.trim()
      zaloStatus = await desktop.saveZaloConfig(update)
      zaloHasToken = zaloStatus.configured
      zaloToken = ''
      zaloPairingCode = zaloStatus.pairing_code ?? ''
      zaloPairedChats = zaloStatus.paired_chat_ids ?? []
      toast.success(zaloStatus.bot_name ? `Zalo đã nối: ${zaloStatus.bot_name}` : 'Đã lưu cấu hình Zalo')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : String(cause))
    } finally {
      zaloSaving = false
    }
  }

  async function runZalo(action: () => Promise<unknown>, ok: string) {
    zaloBusy = true
    try {
      zaloStatus = (await action()) as ZaloStatusInfo
      zaloHasToken = zaloStatus.configured
      zaloPairedChats = zaloStatus.paired_chat_ids ?? []
      zaloPairingCode = zaloStatus.pairing_code ?? ''
      toast.success(ok)
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : String(cause))
    } finally {
      zaloBusy = false
    }
  }

  async function unpairZalo(chatID: string) {
    await runZalo(() => desktop.unpairZaloChat(chatID), `Đã bỏ ghép cặp ${chatID}`)
  }

  async function removeZalo() {
    await runZalo(() => desktop.removeZaloToken(), 'Đã ngắt kết nối Zalo')
    if (!zaloHasToken) zaloEnabled = false
  }
</script>

<div class="fixed inset-0 z-50 bg-black/35 backdrop-blur-sm flex items-center justify-center p-4" role="presentation">
  <div class="settings-card" role="dialog" aria-modal="true" aria-label="Cài đặt Gotack">
    <header class="px-5 py-4 border-b border-mm-border flex items-center justify-between">
      <div>
        <h2 class="text-base font-semibold text-mm-text">Cài đặt</h2>
      </div>
      <button type="button" class="btn-notion px-2 py-1 text-xs" onclick={onClose}>Đóng</button>
    </header>

    <nav class="settings-tabs" aria-label="Mục cài đặt">
      <button
        type="button"
        class="tab-btn"
        class:active={activeTab === 'providers'}
        onclick={() => (activeTab = 'providers')}
      >
        <svg class="tab-icon" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" /></svg>
        Providers
      </button>
      <button
        type="button"
        class="tab-btn"
        class:active={activeTab === 'zalo'}
        onclick={() => (activeTab = 'zalo')}
      >
        <svg class="tab-icon" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" /></svg>
        Zalo
      </button>
      <button
        type="button"
        class="tab-btn"
        class:active={activeTab === 'appearance'}
        onclick={() => (activeTab = 'appearance')}
      >
        <svg class="tab-icon" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 21a4 4 0 01-4-4 4 4 0 014-4h.5a3 3 0 003-3V9a5 5 0 0110 0v2a5 5 0 01-5 5h-1a2 2 0 00-2 2v2a1 1 0 01-1 1H7z" /></svg>
        Giao diện
      </button>
    </nav>

    <div class="p-5 overflow-y-auto flex-1 space-y-6">
      {#if activeTab === 'providers'}
        <section class="setting-section">
          <div class="section-title">Provider đã cấu hình</div>
          {#if catalog.status === 'ready'}
            {#if configuredProviders.length}
              <div class="provider-list">
                {#each configuredProviders as item (item.id)}
                  <div class="provider-row">
                    <div class="provider-name">
                      <strong>{item.name}</strong>
                      <small>{item.credential_kind === 'oauth' ? 'OAuth' : 'API key'}</small>
                    </div>
                    <code class="provider-secret">
                      {#if revealedProviderKeys[item.id] !== undefined}
                        {revealedProviderKeys[item.id] || (item.credential_kind === 'oauth' ? 'OAuth credential' : 'Không có API key')}
                      {:else}
                        ••••••••••••••••
                      {/if}
                    </code>
                    <div class="provider-actions">
                      <button
                        type="button"
                        class="icon-btn"
                        disabled={revealingProvider === item.id || item.credential_kind === 'oauth'}
                        title={item.credential_kind === 'oauth' ? 'Provider dùng OAuth' : revealedProviderKeys[item.id] !== undefined ? 'Ẩn API key' : 'Hiện API key'}
                        aria-label={revealedProviderKeys[item.id] !== undefined ? `Ẩn API key ${item.name}` : `Hiện API key ${item.name}`}
                        onclick={() => void toggleProviderKey(item.id)}
                      >
                        {#if revealedProviderKeys[item.id] !== undefined}
                          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M3 3l18 18M10.6 10.7a2 2 0 002.7 2.7M9.9 4.2A10.9 10.9 0 0112 4c5.5 0 9.5 5.4 9.5 8a7.7 7.7 0 01-2 3.6M6.2 6.2C4 7.7 2.5 10.2 2.5 12c0 2.6 4 8 9.5 8 1.4 0 2.7-.3 3.9-.8" /></svg>
                        {:else}
                          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M2.5 12S6.5 4 12 4s9.5 8 9.5 8-4 8-9.5 8-9.5-8z" /><circle cx="12" cy="12" r="3" /></svg>
                        {/if}
                      </button>
                      <button
                        type="button"
                        class="icon-btn delete-btn"
                        disabled={deletingProvider === item.id}
                        title={`Xóa ${item.name}`}
                        aria-label={`Xóa provider ${item.name}`}
                        onclick={() => void deleteConfiguredProvider(item.id, item.name)}
                      >
                        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M3 6h18M8 6V4h8v2M19 6l-1 14H6L5 6M10 11v5M14 11v5" /></svg>
                      </button>
                    </div>
                  </div>
                {/each}
              </div>
            {:else}
              <p class="hint">Chưa có provider nào được tải thành công. Cấu hình credential bên dưới rồi lưu.</p>
            {/if}

            <label class="field-label" for="provider-select">Thêm / chỉnh provider</label>
            <select id="provider-select" class="field" value={selectedProvider} onchange={(event) => chooseProvider(event.currentTarget.value)} aria-label="Provider">
              <option value="" disabled>Chọn provider</option>
              {#each catalog.providers as item (item.id)}<option value={item.id}>{item.name}</option>{/each}
            </select>
            {#if selectedProvider}
              <label class="field-label" for="endpoint">Custom endpoint (tùy chọn)</label>
              <input id="endpoint" class="field font-mono" bind:value={currentCustomUrl} placeholder={providerInfo?.api_endpoint ?? 'https://…'} />
              <p class="hint">Ghi vào <code>providers.{selectedProvider}.base_url</code> qua API cấu hình của Tack.</p>

              <label class="field-label" for="api-key">API key</label>
              <div class="flex gap-2">
                <input id="api-key" class="field flex-1 font-mono" type={showApiKey ? 'text' : 'password'} bind:value={currentApiKey} autocomplete="off" placeholder="Bỏ trống để giữ credential hiện tại" />
                <button type="button" class="btn-notion px-3 text-xs" onclick={() => (showApiKey = !showApiKey)}>{showApiKey ? 'Ẩn' : 'Hiện'}</button>
              </div>
              <p class="hint">Credential được Tack lưu an toàn. Danh sách phía trên chỉ hiện provider có credential sử dụng được.</p>
            {/if}
          {:else if catalog.status === 'loading'}
            <p class="hint">Đang tải danh sách provider...</p>
          {:else if catalog.status === 'error'}
            <p class="hint">Không tải được provider catalog: {catalog.error}. Tack sẽ tự thử lại khi backend sẵn sàng.</p>
          {/if}
        </section>
      {:else if activeTab === 'zalo'}
        <section class="setting-section">
          <div class="section-title">Zalo</div>
          <label class="toggle-row">
            <span><strong>Kết nối Zalo Bot</strong><small>Nhận yêu cầu và trả kết quả qua Zalo khi bạn vắng mặt.</small></span>
            <input type="checkbox" bind:checked={zaloEnabled} />
          </label>

          <label class="field-label" for="zalo-token">Bot token {zaloHasToken ? '(đã lưu, bỏ trống để giữ)' : ''}</label>
          <input id="zalo-token" class="field font-mono" type="password" bind:value={zaloToken} autocomplete="off" placeholder={zaloHasToken ? '••••••••' : 'Token từ Zalo Bot Platform'} />
          <p class="hint">Tạo bot và lấy token tại <code>bot.zaloplatforms.com</code>. Token chỉ lưu trên máy bạn.</p>

          {#if zaloPairingCode}
            <div class="notice">
              <strong>Mã ghép cặp:</strong> <code class="text-base">{zaloPairingCode}</code>
              <p class="hint">Nhắn <code>/pair {zaloPairingCode}</code> cho bot để ghép cặp chat này với Gotack.</p>
              <button type="button" class="btn-notion text-xs" disabled={zaloBusy} onclick={() => runZalo(() => desktop.regenerateZaloPairingCode(), 'Đã sinh mã pairing mới')}>Sinh mã mới</button>
            </div>
          {/if}

          {#if zaloPairedChats.length}
            <div>
              <div class="field-label">Đã ghép cặp</div>
              <div class="flex flex-wrap gap-2">
                {#each zaloPairedChats as chatID (chatID)}
                  <span class="flex items-center gap-1 px-2 py-1 rounded-md bg-mm-panel border border-mm-border text-xs">
                    <code>{chatID}</code>
                    <button type="button" class="text-red-500 px-1" disabled={zaloBusy} onclick={() => unpairZalo(chatID)} title="Bỏ ghép cặp">×</button>
                  </span>
                {/each}
              </div>
            </div>
          {/if}

          {#if zaloStatus}
            <div class="notice">
              {#if zaloStatus.bot_name}
                <div><strong>Bot:</strong> {zaloStatus.bot_name}{zaloStatus.running ? ' · đang chạy' : ''}{zaloStatus.token_suffix ? ` · ••••${zaloStatus.token_suffix}` : ''}</div>
              {/if}
              {#if zaloStatus.last_error}<div class="text-red-500"><strong>Lỗi:</strong> {zaloStatus.last_error}</div>{/if}
              {#if !zaloStatus.bot_name && !zaloStatus.last_error}<div>Chưa kết nối. Lưu token rồi bật kết nối.</div>{/if}
            </div>
          {/if}

          <div class="flex flex-wrap justify-end gap-2 pt-2">
            <button type="button" class="btn-notion text-xs" disabled={zaloBusy || !zaloHasToken} onclick={() => runZalo(() => desktop.testZaloConnection(), 'Kết nối Zalo thành công')}>Kiểm tra kết nối</button>
            <button type="button" class="btn-danger text-xs" disabled={zaloBusy || !zaloHasToken} onclick={removeZalo}>Ngắt kết nối</button>
            <button type="button" class="px-3 py-1.5 rounded-md bg-mm-accent text-white text-xs font-medium disabled:opacity-40" disabled={zaloSaving || (zaloEnabled && !zaloHasToken && !zaloToken.trim())} onclick={saveZalo}>
              {zaloSaving ? 'Đang lưu...' : 'Lưu & kết nối Zalo'}
            </button>
          </div>
        </section>
      {:else if activeTab === 'appearance'}
        <section class="setting-section">
          <div class="section-title">Giao diện</div>
          <div class="grid grid-cols-3 gap-2">
            {#each ['system', 'light', 'dark'] as value}
              <button
                type="button"
                class:active={selectedTheme === value}
                class="option-btn"
                onclick={() => {
                  selectedTheme = value as Theme
                  onThemeChange(selectedTheme)
                }}
              >
                {value === 'system' ? 'Hệ thống' : value === 'light' ? 'Sáng' : 'Tối'}
              </button>
            {/each}
          </div>
        </section>
      {/if}
    </div>

    <footer class="px-5 py-3 border-t border-mm-border flex items-center justify-end">
      <div class="flex gap-2">
        <button type="button" class="btn-notion px-3 py-1.5 text-xs" onclick={onClose}>Hủy</button>
        <button type="button" class="px-4 py-1.5 rounded-md bg-mm-accent text-white text-xs font-medium" onclick={save}>Lưu & áp dụng</button>
      </div>
    </footer>
  </div>
</div>

<style>
  .settings-card { width: min(720px, calc(100vw - 32px)); min-height: 440px; max-height: min(760px, calc(100vh - 32px)); display: flex; flex-direction: column; overflow: hidden; border: 1px solid var(--mm-border); border-radius: 12px; background: var(--mm-bg); box-shadow: 0 24px 70px rgb(0 0 0 / 24%); }
  .settings-tabs { display: flex; gap: 4px; padding: 0 20px; border-bottom: 1px solid var(--mm-border); background: var(--mm-panel); }
  .tab-btn { display: inline-flex; align-items: center; gap: 7px; padding: 10px 14px; font-size: 13px; font-weight: 500; color: var(--mm-secondary); border: none; border-bottom: 2px solid transparent; background: transparent; cursor: pointer; transition: color 120ms ease, border-color 120ms ease; margin-bottom: -1px; }
  .tab-btn:hover { color: var(--mm-text); }
  .tab-btn.active { color: var(--mm-accent); border-bottom-color: var(--mm-accent); font-weight: 600; }
  .tab-icon { width: 15px; height: 15px; }
  .setting-section { display: grid; gap: 10px; }
  .section-title { font-size: 11px; font-weight: 700; color: var(--mm-tertiary); letter-spacing: .08em; text-transform: uppercase; }
  .field-label { margin-top: 3px; font-size: 12px; font-weight: 600; color: var(--mm-secondary); }
  .field { width: 100%; min-height: 36px; padding: 7px 9px; border: 1px solid var(--mm-border); border-radius: 7px; background: var(--mm-panel); color: var(--mm-text); font-size: 12px; outline: none; }
  .field:focus { border-color: var(--mm-accent); }
  .hint { margin: -3px 0 0; font-size: 11px; line-height: 1.5; color: var(--mm-tertiary); }
  .notice { padding: 9px 10px; border: 1px solid var(--mm-border); border-radius: 7px; background: var(--mm-panel); color: var(--mm-secondary); font-size: 11px; line-height: 1.5; display: grid; gap: 4px; }
  .option-btn { padding: 8px; border: 1px solid var(--mm-border); border-radius: 7px; background: var(--mm-panel); color: var(--mm-secondary); font-size: 12px; cursor: pointer; transition: all 120ms ease; }
  .option-btn:hover { background: var(--mm-hover); color: var(--mm-text); }
  .option-btn.active { border-color: var(--mm-accent); color: var(--mm-text); box-shadow: 0 0 0 1px var(--mm-accent); background: var(--mm-bg); }
  .toggle-row { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 9px 0; font-size: 12px; color: var(--mm-text); }
  .toggle-row span { display: grid; gap: 2px; }
  .toggle-row small { color: var(--mm-tertiary); font-size: 10px; }
  .provider-list { display: grid; gap: 7px; }
  .provider-row { display: grid; grid-template-columns: minmax(130px, 1fr) minmax(180px, 1.4fr) auto; align-items: center; gap: 10px; padding: 8px 9px; border: 1px solid var(--mm-border); border-radius: 7px; background: var(--mm-panel); }
  .provider-name { min-width: 0; display: grid; gap: 1px; }
  .provider-name strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 12px; color: var(--mm-text); }
  .provider-name small { font-size: 10px; color: var(--mm-tertiary); }
  .provider-secret { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 11px; color: var(--mm-secondary); }
  .provider-actions { display: flex; align-items: center; gap: 6px; }
  .icon-btn { width: 30px; height: 30px; display: grid; place-items: center; border: 1px solid var(--mm-border); border-radius: 6px; background: var(--mm-bg); color: var(--mm-secondary); cursor: pointer; }
  .icon-btn:hover:not(:disabled) { color: var(--mm-text); background: var(--mm-hover); }
  .icon-btn:disabled { opacity: .45; cursor: default; }
  .icon-btn svg { width: 15px; height: 15px; fill: none; stroke: currentColor; stroke-width: 1.8; stroke-linecap: round; stroke-linejoin: round; }
  .delete-btn { color: #ef4444; }
  .delete-btn:hover:not(:disabled) { color: #ef4444; background: rgb(239 68 68 / 10%); border-color: rgb(239 68 68 / 35%); }
</style>
