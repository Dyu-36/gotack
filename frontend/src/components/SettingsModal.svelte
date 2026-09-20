<script lang="ts">
  import { toast } from 'svelte-sonner'
  import { untrack } from 'svelte'
  import { catalog } from '../features/conversations/catalog.svelte'
  import { desktop, type AgentSettingsInfo, type ChatGPTOAuthStatus, type WorkspaceInfo, type ZaloConfigUpdate, type ZaloStatusInfo } from '../platform/desktop'

  type Theme = 'system' | 'light' | 'dark'

  type Tab = 'providers' | 'agent' | 'zalo' | 'appearance'

  type SettingsPayload = {
    theme: Theme
    provider: string
    credential_provider?: string
    provider_only?: boolean
    model: string
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
    onSaveSettings?: (settings: SettingsPayload) => Promise<void>
    onClose: () => void
  }

  let {
    theme,
    provider = '',
    model = '',
    thinking = 'high',
    customUrl = '',
    onThemeChange,
    onSaveSettings = async () => {},
    onClose,
  }: Props = $props()

  let activeTab = $state<Tab>('providers')
  let selectedTheme = $state<Theme>('system')
  let selectedProvider = $state('')
  let currentApiKey = $state('')
  let currentCustomUrl = $state('')
  let showApiKey = $state(false)
  let deletingProvider = $state('')

  let agentSettings = $state<AgentSettingsInfo>({ tools: [], disabled_tools: [] })
  let agentBusy = $state(false)
  let workspaceInfo = $state<WorkspaceInfo | null>(null)

  let zaloEnabled = $state(false)
  let zaloToken = $state('')
  let zaloHasToken = $state(false)
  let zaloStatus = $state<ZaloStatusInfo | null>(null)
  let zaloSaving = $state(false)
  let zaloBusy = $state(false)
  let zaloPairingCode = $state('')
  let zaloPairingExpires = $state(0)
  let zaloPairedChats = $state<string[]>([])

  let chatgptOAuthStatus = $state<ChatGPTOAuthStatus | null>(null)
  let isLoggingInChatGPT = $state(false)
  let chatgptOAuthURL = $state('')
  let isCopiedOAuthURL = $state(false)

  let autoStart = $state(false)
  let autoStartBusy = $state(false)

  let hydrated = false
  let savingSettings = $state(false)

  $effect(() => {
    selectedTheme = theme
    selectedProvider = provider
    currentApiKey = ''
    currentCustomUrl = customUrl
    untrack(() => {
      if (hydrated) return
      hydrated = true
      if (catalog.status === 'idle') void catalog.refresh()
      void loadZalo()
      void loadChatGPTOAuthStatus()
      void loadAutoStart()
      void loadAgentSettings()
    })
  })

  async function loadAutoStart() {
    try {
      autoStart = await desktop.getAutoStart()
    } catch {
      autoStart = false
    }
  }

  async function toggleAutoStart() {
    const next = !autoStart
    autoStartBusy = true
    try {
      await desktop.setAutoStart(next)
      autoStart = next
      toast.success(next ? 'Đã bật khởi động cùng Windows' : 'Đã tắt khởi động cùng Windows')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : String(cause))
    } finally {
      autoStartBusy = false
    }
  }

  $effect(() => {
    const unsubUrl = desktop.on<string>('chatgpt:oauth:url', (url: string) => {
      chatgptOAuthURL = url
      isLoggingInChatGPT = true
    })
    const unsubCanceled = desktop.on('chatgpt:oauth:canceled', () => {
      chatgptOAuthURL = ''
      isLoggingInChatGPT = false
    })
    return () => {
      unsubUrl()
      unsubCanceled()
    }
  })

  async function loadChatGPTOAuthStatus() {
    try {
      chatgptOAuthStatus = await desktop.getChatGPTOAuthStatus()
      const url = await desktop.getChatGPTOAuthURL()
      if (url) {
        chatgptOAuthURL = url
        isLoggingInChatGPT = true
      }
    } catch {
      chatgptOAuthStatus = null
    }
  }

  async function loginWithChatGPT() {
    isLoggingInChatGPT = true
    chatgptOAuthURL = ''
    try {
      toast.info('Đang mở trình duyệt để đăng nhập tài khoản ChatGPT...')
      chatgptOAuthStatus = await desktop.loginChatGPTOAuth()
      await catalog.refresh()
      toast.success(
        chatgptOAuthStatus?.email
          ? `Đã liên kết tài khoản ChatGPT: ${chatgptOAuthStatus.email}`
          : 'Đăng nhập ChatGPT OAuth thành công!',
      )
    } catch (cause) {
      const msg = cause instanceof Error ? cause.message : String(cause)
      if (!msg.toLowerCase().includes('canceled')) {
        toast.error(msg)
      }
    } finally {
      isLoggingInChatGPT = false
      chatgptOAuthURL = ''
    }
  }

  async function cancelLoginWithChatGPT() {
    try {
      await desktop.cancelChatGPTOAuth()
      toast.info('Đã hủy phiên đăng nhập ChatGPT')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : String(cause))
    } finally {
      isLoggingInChatGPT = false
      chatgptOAuthURL = ''
    }
  }

  async function copyOAuthURL() {
    if (!chatgptOAuthURL) return
    try {
      await navigator.clipboard.writeText(chatgptOAuthURL)
      isCopiedOAuthURL = true
      toast.success('Đã sao chép link đăng nhập!')
      setTimeout(() => {
        isCopiedOAuthURL = false
      }, 2000)
    } catch {
      toast.error('Không thể sao chép vào clipboard')
    }
  }

  async function logoutChatGPT() {
    if (!window.confirm('Đăng xuất tài khoản ChatGPT và hủy liên kết OAuth?')) return
    try {
      await desktop.logoutChatGPTOAuth()
      chatgptOAuthStatus = { connected: false }
      await catalog.refresh()
      toast.success('Đã đăng xuất tài khoản ChatGPT')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : String(cause))
    }
  }

  async function loadZalo() {

    try {
      const config = await desktop.getZaloConfig()
      zaloEnabled = config.enabled
      zaloHasToken = config.has_token
      zaloPairingCode = config.pairing_code
      zaloPairingExpires = config.pairing_expires_at ?? 0
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

  async function deleteConfiguredProvider(providerID: string, name: string) {
    if (!window.confirm(`Xóa cấu hình provider ${name}? Provider sẽ bị tắt trong Tack và credential đã lưu sẽ bị xóa.`)) return
    deletingProvider = providerID
    try {
      await desktop.deleteProvider(providerID)
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

  async function loadAgentSettings() {
    try {
      agentSettings = await desktop.getAgentSettings()
      workspaceInfo = await desktop.currentWorkspace()
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : String(cause))
    }
  }

  async function toggleAgentTool(tool: string, enabled: boolean) {
    const disabled = new Set(agentSettings.disabled_tools)
    if (enabled) disabled.delete(tool)
    else disabled.add(tool)
    agentBusy = true
    try {
      agentSettings = await desktop.saveAgentSettings([...disabled])
      toast.success('Đã cập nhật tool của agent')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : String(cause))
    } finally {
      agentBusy = false
    }
  }

  async function setProjectTrust(trusted: boolean) {
    agentBusy = true
    try {
      workspaceInfo = await desktop.setWorkspaceTrust(trusted)
      toast.success(trusted ? 'Đã trust project' : 'Đã chặn tài nguyên động của project')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : String(cause))
    } finally {
      agentBusy = false
    }
  }

  async function resetProjectTrust() {
    agentBusy = true
    try {
      workspaceInfo = await desktop.resetWorkspaceTrust()
      toast.success('Đã xóa quyết định trust riêng của project')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : String(cause))
    } finally {
      agentBusy = false
    }
  }

  async function save() {
    if (savingSettings) return
    savingSettings = true
    const payload: SettingsPayload = {
      theme: selectedTheme,
      provider,
      credential_provider: selectedProvider || undefined,
      provider_only: true,
      model,
      thinking,
      api_key: selectedProvider === 'codex' ? '' : currentApiKey.trim(),
      custom_url: selectedProvider && selectedProvider !== 'codex' ? currentCustomUrl.trim() : '',
    }
    try {
      await onSaveSettings(payload)
      onThemeChange(selectedTheme)
      currentApiKey = ''
      onClose()
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : String(cause))
    } finally {
      savingSettings = false
    }
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
      zaloPairingExpires = zaloStatus.pairing_expires_at ?? 0
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
      zaloPairingExpires = zaloStatus.pairing_expires_at ?? 0
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
        class:active={activeTab === 'agent'}
        onclick={() => (activeTab = 'agent')}
      >
        <svg class="tab-icon" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v3m0 12v3M3 12h3m12 0h3M5.6 5.6l2.1 2.1m8.6 8.6 2.1 2.1m0-12.8-2.1 2.1m-8.6 8.6-2.1 2.1M12 8a4 4 0 100 8 4 4 0 000-8z" /></svg>
        Agent
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
                    <code class="provider-secret">{item.credential_kind === 'oauth' ? 'OAuth credential' : '••••••••••••••••'}</code>
                    <div class="provider-actions">
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
              {#if selectedProvider === 'codex'}
                <div class="oauth-box">
                  <div class="oauth-header">
                    <div class="oauth-icon">
                      <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 14.5v-9l6 4.5-6 4.5z"/></svg>
                    </div>
                    <div class="oauth-info">
                      <strong>Xác thực tài khoản ChatGPT (OAuth PKCE)</strong>
                      <p>Sử dụng tài khoản ChatGPT (Free, Go, Plus, Pro, Business, Edu hoặc Enterprise) trực tiếp qua trình duyệt mà không cần tạo OpenAI API Key riêng.</p>
                    </div>
                  </div>
                  {#if isLoggingInChatGPT}
                    <div class="oauth-progress-box">
                      <div class="oauth-waiting-badge">
                        <svg class="animate-spin h-3.5 w-3.5 text-emerald-500" viewBox="0 0 24 24" fill="none"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z"></path></svg>
                        <span>Đang chờ hoàn tất đăng nhập trên trình duyệt...</span>
                      </div>
                      {#if chatgptOAuthURL}
                        <div class="oauth-url-box">
                          <label class="oauth-url-label" for="oauth-link-input">Link đăng nhập (dán vào trình duyệt bạn muốn):</label>
                          <div class="flex gap-2 items-center">
                            <input
                              id="oauth-link-input"
                              type="text"
                              readonly
                              class="field font-mono text-xs flex-1 select-all"
                              value={chatgptOAuthURL}
                              onclick={(e) => e.currentTarget.select()}
                            />
                            <button
                              type="button"
                              class="btn-notion text-xs whitespace-nowrap px-2.5 py-1.5"
                              onclick={copyOAuthURL}
                            >
                              {isCopiedOAuthURL ? '✓ Đã chép' : 'Sao chép link'}
                            </button>
                          </div>
                        </div>
                      {/if}
                      <div class="flex justify-end pt-1">
                        <button
                          type="button"
                          class="btn-notion text-xs text-red-500 hover:text-red-600 px-3 py-1"
                          onclick={cancelLoginWithChatGPT}
                        >
                          Hủy đăng nhập
                        </button>
                      </div>
                    </div>
                  {:else if chatgptOAuthStatus?.connected}
                    <div class="oauth-connected">
                      <div class="oauth-badge">
                        <span class="status-dot"></span>
                        <span>Đã kết nối {chatgptOAuthStatus.email ? `(${chatgptOAuthStatus.email})` : ''} {chatgptOAuthStatus.plan ? `· Gói: ${chatgptOAuthStatus.plan.toUpperCase()}` : ''}</span>
                      </div>
                      <div class="oauth-actions">
                        <button type="button" class="btn-notion text-xs text-red-500 hover:text-red-600" onclick={logoutChatGPT}>Đăng xuất</button>
                        <button type="button" class="btn-notion text-xs" onclick={loginWithChatGPT}>Đăng nhập lại</button>
                      </div>
                    </div>
                  {:else}
                    <div class="oauth-login">
                      <button
                        type="button"
                        class="btn-oauth"
                        onclick={loginWithChatGPT}
                      >
                        <svg viewBox="0 0 24 24" class="w-4 h-4" fill="currentColor"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 14.5v-9l6 4.5-6 4.5z"/></svg>
                        <span>Đăng nhập bằng tài khoản ChatGPT</span>
                      </button>
                    </div>
                  {/if}
                </div>
              {/if}

              {#if selectedProvider === 'codex'}
                <p class="hint">Codex đăng nhập bằng tài khoản ChatGPT nên không dùng API key hay endpoint riêng. Cần OpenAI API Key thì chọn provider <code>OpenAI</code>.</p>
              {:else}
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
            {/if}

          {:else if catalog.status === 'loading'}
            <p class="hint">Đang tải danh sách provider...</p>
          {:else if catalog.status === 'error'}
            <p class="hint">Không tải được provider catalog: {catalog.error}. Tack sẽ tự thử lại khi backend sẵn sàng.</p>
          {/if}
        </section>
      {:else if activeTab === 'agent'}
        <section class="setting-section">
          <div class="section-title">Công cụ agent</div>
          <p class="hint">Mặc định Gotack bật toàn bộ 6 công cụ lõi. Đây là cấu hình capability, không phải cơ chế xin quyền từng lần.</p>
          {#each agentSettings.tools as tool (tool)}
            <label class="toggle-row">
              <span>
                <strong>{tool}</strong>
                <small>{tool === 'powershell' ? 'Thực thi lệnh hệ thống với quyền của người dùng đang chạy Gotack.' : 'Công cụ lõi của agent.'}</small>
              </span>
              <input
                type="checkbox"
                checked={!agentSettings.disabled_tools.includes(tool)}
                disabled={agentBusy}
                onchange={(event) => void toggleAgentTool(tool, event.currentTarget.checked)}
              />
            </label>
          {/each}

          <div class="section-title mt-3">Project Trust</div>
          {#if workspaceInfo}
            <div class="notice">
              <div><strong>Workspace:</strong> <code>{workspaceInfo.path}</code></div>
              <div><strong>Trạng thái:</strong> {workspaceInfo.trusted ? 'Trusted' : workspaceInfo.trust_required ? 'Chưa quyết định' : 'Không trusted'}</div>
              {#if workspaceInfo.protected_resources?.length}
                <p class="hint">Tài nguyên động: {workspaceInfo.protected_resources.join(', ')}</p>
              {:else}
                <p class="hint">Workspace không có tài nguyên động cần trust.</p>
              {/if}
            </div>
            <div class="flex flex-wrap justify-end gap-2">
              <button type="button" class="btn-notion text-xs" disabled={agentBusy} onclick={() => void setProjectTrust(false)}>Không trust</button>
              <button type="button" class="btn-notion text-xs" disabled={agentBusy} onclick={() => void resetProjectTrust()}>Kế thừa</button>
              <button type="button" class="px-3 py-1.5 rounded-md bg-mm-accent text-white text-xs font-medium disabled:opacity-40" disabled={agentBusy} onclick={() => void setProjectTrust(true)}>Trust project</button>
            </div>
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

          {#if zaloPairingCode && zaloPairingExpires}
  <p class="hint">Mã ghép cặp hết hạn lúc {new Date(zaloPairingExpires * 1000).toLocaleString('vi-VN')}. Tạo mã mới nếu mã đã hết hạn.</p>
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

          <div class="section-title">Ứng dụng</div>
          <label class="toggle-row">
            <span>
              <strong>Khởi động cùng Windows</strong>
              <small>Tack chạy ẩn trong khay hệ thống ngay khi đăng nhập để Zalo bot luôn trực.</small>
            </span>
            <input type="checkbox" checked={autoStart} disabled={autoStartBusy} onchange={toggleAutoStart} />
          </label>
          <p class="hint">Trên Windows, đóng cửa sổ chỉ ẩn Gotack xuống khay; Zalo tiếp tục hoạt động khi được bật. Chọn Quit Gotack trong khay hệ thống để thoát và dừng engine.</p>
        </section>
      {/if}
    </div>

    <footer class="px-5 py-3 border-t border-mm-border flex items-center justify-end">
      <div class="flex gap-2">
        <button type="button" class="btn-notion px-3 py-1.5 text-xs" onclick={onClose}>Hủy</button>
        <button type="button" class="px-4 py-1.5 rounded-md bg-mm-accent text-white text-xs font-medium" onclick={save} disabled={savingSettings}>Lưu & áp dụng</button>
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
  .oauth-box { padding: 12px; border: 1px solid var(--mm-border); border-radius: 8px; background: var(--mm-panel); display: grid; gap: 8px; }
  .oauth-header { display: flex; align-items: flex-start; gap: 10px; }
  .oauth-icon { width: 22px; height: 22px; color: #10b981; flex-shrink: 0; margin-top: 1px; }
  .oauth-icon svg { width: 100%; height: 100%; }
  .oauth-info strong { font-size: 12px; color: var(--mm-text); display: block; }
  .oauth-info p { margin: 2px 0 0; font-size: 11px; color: var(--mm-secondary); line-height: 1.4; }
  .oauth-connected { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding-top: 8px; border-top: 1px solid var(--mm-border); }
  .oauth-badge { display: flex; align-items: center; gap: 6px; font-size: 11px; font-weight: 500; color: #10b981; }
  .status-dot { width: 7px; height: 7px; border-radius: 50%; background: #10b981; }
  .oauth-actions { display: flex; gap: 6px; }
  .oauth-login { padding-top: 4px; }
  .btn-oauth { width: 100%; padding: 8px 12px; border-radius: 6px; border: 1px solid rgba(16, 185, 129, 0.35); background: rgba(16, 185, 129, 0.1); color: #10b981; font-size: 12px; font-weight: 600; cursor: pointer; display: flex; align-items: center; justify-content: center; gap: 8px; transition: all 120ms ease; }
  .btn-oauth:hover:not(:disabled) { background: rgba(16, 185, 129, 0.18); border-color: rgba(16, 185, 129, 0.5); }
  .btn-oauth:disabled { opacity: 0.6; cursor: wait; }
  .oauth-progress-box { display: grid; gap: 8px; padding-top: 4px; }
  .oauth-waiting-badge { display: flex; align-items: center; gap: 8px; font-size: 12px; color: var(--mm-text); }
  .oauth-url-box { display: grid; gap: 4px; padding: 8px; border-radius: 6px; background: var(--mm-bg); border: 1px solid var(--mm-border); }
  .oauth-url-label { font-size: 11px; font-weight: 500; color: var(--mm-secondary); }
</style>
