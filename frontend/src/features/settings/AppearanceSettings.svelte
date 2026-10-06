<script lang="ts">
  import { toast } from 'svelte-sonner'
  import { onMount } from 'svelte'
  import { t } from '../../lib/i18n.svelte'
  import { desktop } from '../../platform/desktop'
  import type { Theme } from './types'

  let { selectedTheme = $bindable<Theme>('system'), onThemeChange }: {
    selectedTheme?: Theme
    onThemeChange: (theme: Theme) => void
  } = $props()

  let autoStart = $state(false)
  let autoStartBusy = $state(false)
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
      toast.success(next ? t('appearance.toastAutoStartOn') : t('appearance.toastAutoStartOff'))
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : String(cause))
    } finally {
      autoStartBusy = false
    }
  }
  onMount(() => { void loadAutoStart() })
</script>

<section class="setting-section">
  <div class="section-title">{t('appearance.title')}</div>
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
        {value === 'system' ? t('appearance.system') : value === 'light' ? t('appearance.light') : t('appearance.dark')}
      </button>
    {/each}
  </div>

  <div class="section-title">{t('appearance.appTitle')}</div>
  <label class="toggle-row">
    <span>
      <strong>{t('appearance.autoStart')}</strong>
      <small>{t('appearance.autoStartHint')}</small>
    </span>
    <input type="checkbox" checked={autoStart} disabled={autoStartBusy} onchange={toggleAutoStart} />
  </label>
  <p class="hint">{t('appearance.trayHint')}</p>
</section>
