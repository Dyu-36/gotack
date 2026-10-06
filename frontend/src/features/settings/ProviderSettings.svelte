<script lang="ts">
  import { toast } from 'svelte-sonner'
  import { onMount } from 'svelte'
  import { catalog } from '../conversations/catalog.svelte'
  import { desktop, type ChatGPTOAuthStatus } from '../../platform/desktop'

  let { provider, customUrl, selectedProvider = $bindable(''), currentApiKey = $bindable(''), currentCustomUrl = $bindable('') }: {
    provider: string
    customUrl: string
    selectedProvider?: string
    currentApiKey?: string
    currentCustomUrl?: string
  } = $props()

  let showApiKey = $state(false)
  let deletingProvider = $state('')
  let chatgptOAuthStatus = $state<ChatGPTOAuthStatus | null>(null)
  let isLoggingInChatGPT = $state(false)
  let chatgptOAuthURL = $state('')
  let isCopiedOAuthURL = $state(false)
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
  onMount(() => { void loadChatGPTOAuthStatus() })
</script>

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

<style>
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
