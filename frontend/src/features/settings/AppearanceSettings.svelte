<script lang="ts">
  import { toast } from 'svelte-sonner'
  import { onMount } from 'svelte'
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
      toast.success(next ? 'Đã bật khởi động cùng Windows' : 'Đã tắt khởi động cùng Windows')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : String(cause))
    } finally {
      autoStartBusy = false
    }
  }
  onMount(() => { void loadAutoStart() })
</script>

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
