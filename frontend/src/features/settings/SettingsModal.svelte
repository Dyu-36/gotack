<script lang="ts">
  import { toast } from 'svelte-sonner'
  import { untrack } from 'svelte'
  import { catalog } from '../conversations/catalog.svelte'
  import ProviderSettings from './ProviderSettings.svelte'
  import AgentSettings from './AgentSettings.svelte'
  import ZaloSettings from './ZaloSettings.svelte'
  import AppearanceSettings from './AppearanceSettings.svelte'
  import type { Theme, SettingsPayload } from './types'
  import './settings.css'

  type Tab = 'providers' | 'agent' | 'zalo' | 'appearance'

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
  let hydrated = false
  let savingSettings = $state(false)

  $effect(() => {
    selectedTheme = theme
  })

  $effect(() => {
    selectedProvider = provider
    currentApiKey = ''
    currentCustomUrl = customUrl
    untrack(() => {
      if (hydrated) return
      hydrated = true
      if (catalog.status === 'idle') void catalog.refresh()
    })
  })
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

    <div class="p-5 overflow-y-auto flex-1">
      <!-- Keep panels mounted so drafts and OAuth state survive tab changes. -->
      <div hidden={activeTab !== 'providers'}>
        <ProviderSettings {provider} {customUrl} bind:selectedProvider bind:currentApiKey bind:currentCustomUrl />
      </div>
      <div hidden={activeTab !== 'agent'}><AgentSettings /></div>
      <div hidden={activeTab !== 'zalo'}><ZaloSettings /></div>
      <div hidden={activeTab !== 'appearance'}><AppearanceSettings bind:selectedTheme {onThemeChange} /></div>
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
</style>
