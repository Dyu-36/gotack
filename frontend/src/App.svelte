<script lang="ts">
  import { Toaster } from 'svelte-sonner'
  import { callDesktop, hasDesktopBridge } from './lib/desktop'

  type Session = {
    id: string
    title: string
    updatedAt: number
    pinned?: boolean
  }

  const seedSessions: Session[] = [
    { id: 'welcome', title: 'Bắt đầu với Gotack', updatedAt: Date.now(), pinned: true },
    { id: 'workspace', title: 'Phân tích workspace', updatedAt: Date.now() - 3600000 },
  ]

  let sessions = $state(seedSessions)
  let activeSessionId = $state('welcome')
  let sidebarOpen = $state(true)
  let settingsOpen = $state(false)
  let workspace = $state('Chưa chọn workspace')
  let input = $state('')
  let searching = $state('')
  let theme = $state<'light' | 'dark' | 'system'>('system')
  let backendReady = $state(false)
  let messages = $state([
    {
      role: 'assistant',
      content: 'Gotack đang dùng UI desktop kế thừa từ Stack. Backend Wails/Crush sẽ được nối qua bridge Go ở bước tiếp theo.',
    },
  ])

  let filteredSessions = $derived(
    sessions.filter((session) => session.title.toLowerCase().includes(searching.toLowerCase())),
  )

  $effect(() => {
    if (typeof window === 'undefined') return
    const saved = localStorage.getItem('gotack.theme') as typeof theme | null
    if (saved) theme = saved
    applyTheme(theme)
    if (hasDesktopBridge()) {
      void callDesktop<boolean>('BackendReady').then((value) => (backendReady = value)).catch(() => {})
    }
  })

  function applyTheme(value: typeof theme) {
    const dark = value === 'dark' || (value === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches)
    document.documentElement.classList.toggle('dark', dark)
    document.documentElement.dataset.theme = dark ? 'dark' : 'light'
    document.documentElement.style.colorScheme = dark ? 'dark' : 'light'
    localStorage.setItem('gotack.theme', value)
  }

  function setTheme(value: typeof theme) {
    theme = value
    applyTheme(value)
  }

  function newSession() {
    const id = crypto.randomUUID()
    sessions = [{ id, title: 'Hội thoại mới', updatedAt: Date.now() }, ...sessions]
    activeSessionId = id
    messages = []
  }

  function send() {
    const content = input.trim()
    if (!content) return
    messages = [...messages, { role: 'user', content }]
    input = ''
    messages = [
      ...messages,
      {
        role: 'assistant',
        content: backendReady
          ? 'UI đã kết nối Wails. Phần Crush streaming/session API chưa được triển khai.'
          : 'Đang chạy frontend preview; Wails backend chưa được attach.',
      },
    ]
  }

  function handleInputKeydown(event: KeyboardEvent) {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault()
      send()
    }
  }
</script>

<div class="app-shell">
  <div class="workspace-frame" style={`--sidebar-col: ${sidebarOpen ? 'var(--mm-sidebar-w)' : '0px'}`}>
    <aside class="sidebar" aria-label="Danh sách hội thoại">
      <div class="sidebar-head" data-wails-drag-region>
        <div class="brand">
          <div class="brand-mark">G</div>
          <span>Gotack</span>
        </div>
        <button class="icon-btn" title="Thu gọn sidebar" onclick={() => (sidebarOpen = false)}>
          <svg viewBox="0 0 24 24"><path d="M11 19l-7-7 7-7M18 19l-7-7 7-7" /></svg>
        </button>
      </div>

      <div class="sidebar-pad">
        <button class="new-chat" onclick={newSession}>
          <svg viewBox="0 0 24 24"><path d="M12 4v16m8-8H4" /></svg>
          Hội thoại mới
        </button>
      </div>

      <div class="sidebar-pad compact">
        <button class="workspace-card" title={workspace}>
          <svg viewBox="0 0 24 24"><path d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" /></svg>
          <div class="workspace-copy">
            <span>Workspace</span>
            <strong>{workspace}</strong>
          </div>
          <svg viewBox="0 0 24 24"><path d="M9 5l7 7-7 7" /></svg>
        </button>
      </div>

      <div class="sidebar-pad compact">
        <div class="search-wrap">
          <svg viewBox="0 0 24 24"><path d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" /></svg>
          <input bind:value={searching} placeholder="Tìm kiếm..." aria-label="Tìm hội thoại" />
        </div>
      </div>

      <div class="divider"></div>
      <div class="session-list">
        {#each filteredSessions as session (session.id)}
          <button
            class:active={activeSessionId === session.id}
            class="session-item"
            onclick={() => (activeSessionId = session.id)}
          >
            <svg viewBox="0 0 24 24"><path d={session.pinned ? 'M5 5a2 2 0 012-2h10a2 2 0 012 2v16l-7-3.5L5 21V5z' : 'M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z'} /></svg>
            <div>
              <strong>{session.title}</strong>
              <span>{new Date(session.updatedAt).toLocaleTimeString('vi', { hour: '2-digit', minute: '2-digit' })}</span>
            </div>
          </button>
        {/each}
      </div>

      <div class="sidebar-footer">
        <button class="icon-btn" title="Cài đặt" onclick={() => (settingsOpen = true)}>
          <svg viewBox="0 0 24 24"><path d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" /><path d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" /></svg>
        </button>
      </div>
    </aside>

    <main class="chat-shell">
      {#if !sidebarOpen}
        <button class="sidebar-reopen icon-btn" title="Mở sidebar" onclick={() => (sidebarOpen = true)}>
          <svg viewBox="0 0 24 24"><path d="M13 5l7 7-7 7M6 5l7 7-7 7" /></svg>
        </button>
      {/if}

      <div class="chat-head" data-wails-drag-region>
        <div>
          <strong>{sessions.find((s) => s.id === activeSessionId)?.title ?? 'Gotack'}</strong>
          <span class:online={backendReady}>{backendReady ? 'Wails backend connected' : 'Frontend preview'}</span>
        </div>
      </div>

      <section class="message-list">
        {#if messages.length === 0}
          <div class="welcome">
            <div class="welcome-mark">G</div>
            <h1>Gotack AI Assistant</h1>
            <p>Crush-powered desktop coding agent with timetable skills, Office CLI and Zalo integration.</p>
          </div>
        {:else}
          <div class="message-column">
            {#each messages as message}
              <article class:user={message.role === 'user'} class="message-row">
                <div class:assistant={message.role === 'assistant'} class:userbubble={message.role === 'user'} class="bubble">
                  {message.content}
                </div>
              </article>
            {/each}
          </div>
        {/if}
      </section>

      <div class="composer-wrap">
        <div class="composer">
          <textarea bind:value={input} onkeydown={handleInputKeydown} placeholder="Nhập tin nhắn... (Enter để gửi, Shift+Enter để xuống dòng)" rows="1"></textarea>
          <div class="composer-bar">
            <div class="composer-actions">
              <button class="icon-btn" title="Đính kèm tệp">
                <svg viewBox="0 0 24 24"><path d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828L18 9.828a4 4 0 00-5.657-5.657L5.757 10.757a6 6 0 108.486 8.486L20.5 13" /></svg>
              </button>
              <button class="model-pill">Crush <svg viewBox="0 0 24 24"><path d="M6 9l6 6 6-6" /></svg></button>
            </div>
            <button class="send-btn" disabled={!input.trim()} onclick={send} title="Gửi">
              <svg viewBox="0 0 24 24"><path d="M12 19V5M5 12l7-7 7 7" /></svg>
            </button>
          </div>
        </div>
      </div>
    </main>
  </div>

  {#if settingsOpen}
    <div class="modal-backdrop" role="presentation" onclick={() => (settingsOpen = false)}>
      <section class="settings-modal" role="dialog" aria-modal="true" aria-label="Cài đặt" onclick={(event) => event.stopPropagation()}>
        <header>
          <div><h2>Cài đặt</h2><p>Desktop preferences và integration shell.</p></div>
          <button class="icon-btn" onclick={() => (settingsOpen = false)}><svg viewBox="0 0 24 24"><path d="M6 6l12 12M18 6L6 18" /></svg></button>
        </header>
        <div class="settings-section">
          <h3>Giao diện</h3>
          <div class="theme-grid">
            <button class:active={theme === 'system'} onclick={() => setTheme('system')}>Hệ thống</button>
            <button class:active={theme === 'light'} onclick={() => setTheme('light')}>Sáng</button>
            <button class:active={theme === 'dark'} onclick={() => setTheme('dark')}>Tối</button>
          </div>
        </div>
        <div class="settings-section">
          <h3>Desktop upgrades</h3>
          <div class="feature-card"><strong>Timetable Skills</strong><span>Custom timetable workflow.</span></div>
          <div class="feature-card"><strong>Office CLI</strong><span>Word, Excel và PowerPoint workflows.</span></div>
          <div class="feature-card"><strong>Zalo</strong><span>Remote control channel.</span></div>
        </div>
      </section>
    </div>
  {/if}

  <Toaster theme={theme === 'system' ? 'system' : theme} position="bottom-right" richColors closeButton />
</div>

<style>
  :global(button), :global(textarea), :global(input) { font: inherit; }
  :global(button) { color: inherit; }
  :global(svg) { width: 18px; height: 18px; fill: none; stroke: currentColor; stroke-width: 1.8; stroke-linecap: round; stroke-linejoin: round; }
  .app-shell { width: 100%; height: 100%; background: var(--tack-app-bg); }
  .workspace-frame { display: grid; grid-template-columns: var(--sidebar-col) minmax(0, 1fr); width: 100%; height: 100%; transition: grid-template-columns 140ms ease; }
  .sidebar { width: var(--mm-sidebar-w); height: 100%; background: var(--mm-sidebar); border-right: 1px solid var(--mm-border); display: flex; flex-direction: column; overflow: hidden; }
  .sidebar-head { display: flex; align-items: center; justify-content: space-between; padding: 12px 12px 6px; }
  .brand { display: flex; align-items: center; gap: 8px; font-weight: 650; }
  .brand-mark, .welcome-mark { display: grid; place-items: center; background: var(--mm-inverse-surface); color: var(--mm-inverse-text); font-weight: 750; border-radius: 8px; }
  .brand-mark { width: 28px; height: 28px; }
  .welcome-mark { width: 52px; height: 52px; font-size: 22px; margin: auto; }
  .icon-btn { display: grid; place-items: center; width: 30px; height: 30px; border: 0; border-radius: 7px; background: transparent; color: var(--mm-secondary); cursor: pointer; }
  .icon-btn:hover { background: var(--mm-hover); color: var(--mm-text); }
  .sidebar-pad { padding: 8px 10px 4px; }
  .sidebar-pad.compact { padding-top: 4px; }
  .new-chat { width: 100%; height: 54px; display: flex; align-items: center; justify-content: center; gap: 10px; border: 0; border-radius: 8px; background: transparent; color: var(--mm-text); font-weight: 550; cursor: pointer; }
  .new-chat:hover { background: var(--mm-hover); }
  .workspace-card { width: 100%; display: flex; align-items: center; gap: 8px; padding: 8px 10px; border: 1px solid color-mix(in srgb, var(--mm-border) 70%, transparent); border-radius: 7px; background: var(--mm-panel); cursor: pointer; text-align: left; }
  .workspace-card > svg { width: 15px; height: 15px; color: var(--mm-secondary); flex: 0 0 auto; }
  .workspace-copy { min-width: 0; flex: 1; display: flex; flex-direction: column; }
  .workspace-copy span { font-size: 10px; text-transform: uppercase; letter-spacing: .08em; color: var(--mm-tertiary); font-weight: 700; }
  .workspace-copy strong { font-size: 12px; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; font-family: var(--font-mono); }
  .search-wrap { position: relative; }
  .search-wrap svg { position: absolute; left: 10px; top: 50%; transform: translateY(-50%); width: 14px; height: 14px; color: var(--mm-secondary); }
  .search-wrap input { width: 100%; height: 32px; padding: 0 10px 0 32px; border: 1px solid transparent; border-radius: 7px; background: var(--mm-panel); color: var(--mm-text); outline: none; }
  .search-wrap input:focus { border-color: var(--mm-border); background: var(--mm-bg); }
  .divider { border-top: 1px solid var(--mm-border); margin: 6px 12px; }
  .session-list { flex: 1; overflow-y: auto; padding: 0 6px 8px; }
  .session-item { width: 100%; display: flex; align-items: center; gap: 9px; padding: 8px 10px; border: 0; border-radius: 7px; background: transparent; cursor: pointer; text-align: left; }
  .session-item:hover, .session-item.active { background: var(--mm-hover); }
  .session-item > svg { width: 15px; height: 15px; color: var(--mm-secondary); flex: 0 0 auto; }
  .session-item > div { min-width: 0; display: flex; flex-direction: column; }
  .session-item strong { font-size: 13px; font-weight: 550; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .session-item span { font-size: 11px; color: var(--mm-tertiary); }
  .sidebar-footer { border-top: 1px solid var(--mm-border); padding: 9px 12px; display: flex; justify-content: flex-end; }
  .chat-shell { min-width: 0; position: relative; display: flex; flex-direction: column; background: var(--mm-bg); }
  .chat-head { height: 54px; display: flex; align-items: center; padding: 0 22px; border-bottom: 1px solid var(--mm-border); }
  .chat-head > div { display: flex; flex-direction: column; }
  .chat-head strong { font-size: 13px; }
  .chat-head span { font-size: 10px; color: var(--mm-tertiary); }
  .chat-head span.online { color: var(--mm-success); }
  .sidebar-reopen { position: absolute; z-index: 5; left: 10px; top: 12px; }
  .message-list { flex: 1; overflow-y: auto; min-height: 0; }
  .welcome { height: 100%; display: grid; place-content: center; text-align: center; max-width: 520px; margin: auto; padding: 40px 24px 140px; }
  .welcome h1 { margin: 16px 0 7px; font-size: 22px; }
  .welcome p { margin: 0; color: var(--mm-secondary); line-height: 1.6; }
  .message-column { width: min(756px, calc(100% - 48px)); margin: 0 auto; padding: 28px 0 150px; display: flex; flex-direction: column; gap: 18px; }
  .message-row { display: flex; }
  .message-row.user { justify-content: flex-end; }
  .bubble { max-width: 82%; white-space: pre-wrap; line-height: 1.6; }
  .bubble.assistant { max-width: 100%; }
  .bubble.userbubble { background: var(--mm-user-bubble); color: white; border-radius: 16px; padding: 9px 13px; }
  .composer-wrap { position: absolute; left: 0; right: 0; bottom: 0; padding: 12px 24px 18px; background: linear-gradient(to bottom, transparent, var(--mm-bg) 24%); }
  .composer { width: min(756px, 100%); margin: auto; border: 1px solid var(--mm-border); border-radius: 18px; background: var(--mm-bg); box-shadow: 0 1px 5px rgb(0 0 0 / .06); }
  .composer:focus-within { border-color: color-mix(in srgb, var(--mm-accent) 60%, var(--mm-border)); box-shadow: 0 0 0 2px color-mix(in srgb, var(--mm-accent) 12%, transparent); }
  .composer textarea { width: 100%; resize: none; min-height: 58px; max-height: var(--composer-max-h); padding: 15px 16px 8px; border: 0; outline: 0; background: transparent; color: var(--mm-text); line-height: 1.55; }
  .composer textarea::placeholder { color: var(--mm-tertiary); }
  .composer-bar { display: flex; align-items: center; justify-content: space-between; padding: 4px 10px 10px; }
  .composer-actions { display: flex; align-items: center; gap: 3px; }
  .model-pill { height: 30px; display: inline-flex; align-items: center; gap: 5px; border: 0; border-radius: 7px; padding: 0 9px; background: transparent; color: var(--mm-secondary); cursor: pointer; }
  .model-pill:hover { background: var(--mm-hover); color: var(--mm-text); }
  .model-pill svg { width: 13px; height: 13px; }
  .send-btn { width: 32px; height: 32px; display: grid; place-items: center; border: 0; border-radius: 9px; background: var(--mm-inverse-surface); color: var(--mm-inverse-text); cursor: pointer; }
  .send-btn:disabled { background: var(--mm-hover); color: var(--mm-tertiary); cursor: not-allowed; }
  .modal-backdrop { position: fixed; inset: 0; z-index: 40; display: grid; place-items: center; background: rgb(0 0 0 / .34); padding: 24px; }
  .settings-modal { width: min(620px, 100%); max-height: min(680px, 90vh); overflow: auto; border: 1px solid var(--mm-border); border-radius: 14px; background: var(--mm-bg); box-shadow: var(--shadow-panel-md); }
  .settings-modal header { display: flex; justify-content: space-between; align-items: flex-start; padding: 20px 22px 14px; border-bottom: 1px solid var(--mm-border); }
  .settings-modal h2 { margin: 0; font-size: 18px; }
  .settings-modal header p { margin: 3px 0 0; color: var(--mm-secondary); font-size: 12px; }
  .settings-section { padding: 18px 22px; border-bottom: 1px solid var(--mm-border); }
  .settings-section:last-child { border-bottom: 0; }
  .settings-section h3 { margin: 0 0 12px; font-size: 12px; text-transform: uppercase; letter-spacing: .06em; color: var(--mm-secondary); }
  .theme-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; }
  .theme-grid button { border: 1px solid var(--mm-border); border-radius: 8px; padding: 9px; background: var(--mm-panel); cursor: pointer; }
  .theme-grid button.active { border-color: var(--mm-accent); box-shadow: 0 0 0 1px var(--mm-accent); }
  .feature-card { display: flex; justify-content: space-between; gap: 16px; padding: 10px 0; }
  .feature-card span { color: var(--mm-secondary); text-align: right; }
</style>
