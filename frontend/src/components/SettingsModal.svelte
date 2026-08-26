<script lang="ts">
  type Theme = 'system' | 'light' | 'dark'

  type Props = {
    theme: Theme
    onThemeChange: (theme: Theme) => void
    onClose: () => void
  }

  let { theme, onThemeChange, onClose }: Props = $props()
  let activeTab = $state<'general' | 'session'>('general')
  let autoApprove = $state(false)
  let saved = $state(false)

  const tabs = [
    { id: 'general', label: 'Chung', icon: 'M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6' },
    { id: 'session', label: 'Phiên chat', icon: 'M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z' },
  ] as const

  function save() {
    saved = true
    setTimeout(() => (saved = false), 1600)
  }
</script>

<div class="fixed inset-0 z-40 bg-black/30 dark:bg-black/50 backdrop-blur-sm animate-fade-in">
  <button type="button" class="absolute inset-0 cursor-default" aria-label="Đóng cài đặt" onclick={onClose}></button>
  <section class="pointer-events-auto absolute inset-0 flex items-center justify-center p-4">
    <div class="settings-shell bg-mm-bg rounded-xl border border-mm-border w-full max-w-2xl max-h-[85vh] flex flex-col animate-slide-up" role="dialog" aria-modal="true" aria-label="Cài đặt">
      <div class="flex items-center justify-between px-6 py-4 border-b border-mm-border">
        <h2 class="text-base font-semibold text-mm-text">Cài đặt</h2>
        <button class="btn-notion p-1 text-mm-secondary hover:text-mm-text" aria-label="Đóng" onclick={onClose}>
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
        </button>
      </div>

      <div class="flex flex-1 overflow-hidden min-h-0">
        <div class="w-44 shrink-0 border-r border-mm-border p-2">
          {#each tabs as tab (tab.id)}
            <button class:active={activeTab === tab.id} class="settings-nav w-full mb-0.5" onclick={() => (activeTab = tab.id)}>
              <svg class="w-4 h-4 text-mm-secondary" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={tab.icon} /></svg>
              {tab.label}
            </button>
          {/each}
        </div>

        <div class="flex-1 overflow-y-auto p-6 space-y-6">
          {#if activeTab === 'general'}
            <section>
              <h3 class="text-sm font-semibold text-mm-text mb-1">Giao diện</h3>
              <p class="text-xs text-mm-secondary mb-3">Chọn giao diện cho ứng dụng Gotack.</p>
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
            </section>

            <section class="border-t border-mm-border pt-5">
              <h3 class="text-sm font-semibold text-mm-text mb-1">Quyền thực thi</h3>
              <p class="text-xs text-mm-secondary mb-3">Cấu hình mặc định cho các hành động của coding agent.</p>
              <label class="setting-row">
                <div>
                  <strong>Tự động phê duyệt</strong>
                  <span>Cho phép agent chạy các hành động đã được tin cậy.</span>
                </div>
                <input type="checkbox" bind:checked={autoApprove} class="switch-input" />
              </label>
            </section>

            <section class="border-t border-mm-border pt-5">
              <h3 class="text-sm font-semibold text-mm-text mb-3">Desktop upgrades</h3>
              <div class="space-y-2">
                <div class="feature-row"><div class="feature-icon">📅</div><div><strong>Timetable Skills</strong><span>Workflow thời khóa biểu riêng, có thể customize.</span></div></div>
                <div class="feature-row"><div class="feature-icon">📄</div><div><strong>Office CLI</strong><span>Word, Excel và PowerPoint workflow trên desktop.</span></div></div>
                <div class="feature-row"><div class="feature-icon">💬</div><div><strong>Zalo</strong><span>Kênh tích hợp điều khiển và nhận thông báo.</span></div></div>
              </div>
            </section>
          {:else}
            <section>
              <h3 class="text-sm font-semibold text-mm-text mb-1">Mô hình phiên chat</h3>
              <p class="text-xs text-mm-secondary mb-4">UI cấu hình model được giữ theo Stack; kết nối Crush sẽ thực hiện ở lớp backend sau.</p>

              <div class="form-block">
                <label for="provider">Provider</label>
                <select id="provider"><option>Crush</option></select>
              </div>
              <div class="form-block">
                <label for="model">Model</label>
                <select id="model"><option>Default model</option></select>
              </div>
              <div class="form-block">
                <label for="reasoning">Thinking</label>
                <select id="reasoning"><option>Auto</option><option>Low</option><option>Medium</option><option>High</option></select>
              </div>

              <div class="mt-5 rounded-lg border border-mm-border bg-mm-panel p-4">
                <div class="flex items-start gap-3">
                  <div class="provider-mark">C</div>
                  <div class="flex-1 min-w-0"><div class="text-sm font-semibold text-mm-text">Crush backend</div><div class="text-xs text-mm-secondary mt-1">Provider authentication và model catalog sẽ được nối qua Wails sau khi khóa xong UI.</div></div>
                  <span class="text-2xs rounded-pill border border-mm-border bg-mm-bg px-2 py-1 text-mm-tertiary">UI only</span>
                </div>
              </div>
            </section>
          {/if}
        </div>
      </div>

      <div class="flex items-center justify-between px-6 py-4 border-t border-mm-border">
        <span class="text-xs text-mm-secondary">{saved ? 'Đã lưu' : ''}</span>
        <div class="flex items-center gap-3">
          <button class="btn-notion text-mm-secondary hover:text-mm-text" onclick={onClose}>Hủy</button>
          <button class="btn-primary" onclick={save}>Lưu cài đặt</button>
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
  .setting-row strong, .feature-row strong { display: block; font-size: 13px; color: var(--mm-text); }
  .setting-row span, .feature-row span { display: block; margin-top: 2px; font-size: 11px; color: var(--mm-secondary); }
  .switch-input { width: 34px; height: 18px; accent-color: var(--mm-accent); }
  .feature-row { display: flex; align-items: center; gap: 10px; padding: 10px 12px; border: 1px solid var(--mm-border); border-radius: 8px; background: var(--mm-panel); }
  .feature-icon { width: 30px; height: 30px; display: grid; place-items: center; border: 1px solid var(--mm-border); border-radius: 7px; background: var(--mm-bg); }
  .form-block { margin-bottom: 14px; }
  .form-block label { display: block; margin-bottom: 6px; color: var(--mm-secondary); font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: .04em; }
  .form-block select { width: 100%; height: 36px; padding: 0 10px; border: 1px solid var(--mm-border); border-radius: 7px; background: var(--mm-panel); color: var(--mm-text); font: inherit; font-size: 13px; }
  .provider-mark { width: 34px; height: 34px; display: grid; place-items: center; border-radius: 8px; background: var(--mm-inverse-surface, #322f29); color: var(--mm-inverse-text, white); font-weight: 750; }
</style>
