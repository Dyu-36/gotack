<script lang="ts">
  import { Toaster } from 'svelte-sonner'
  import { callDesktop, hasDesktopBridge } from './lib/desktop'
  import Sidebar from './components/Sidebar.svelte'
  import ChatArea from './components/ChatArea.svelte'
  import SettingsModal from './components/SettingsModal.svelte'

  type Theme = 'light' | 'dark' | 'system'
  type Session = {
    id: string
    title: string
    updatedAt: number
    pinned?: boolean
    streaming?: boolean
  }
  type Message = {
    role: 'user' | 'assistant'
    content: string
  }

  const seedSessions: Session[] = [
    { id: 'welcome', title: 'Bắt đầu với Gotack', updatedAt: Date.now(), pinned: true },
    { id: 'workspace', title: 'Phân tích workspace', updatedAt: Date.now() - 3_600_000 },
    { id: 'crush', title: 'Crush coding session', updatedAt: Date.now() - 86_400_000 },
  ]

  let sessions = $state(seedSessions)
  let activeSessionId = $state('welcome')
  let sidebarOpen = $state(true)
  let settingsOpen = $state(false)
  let workspace = $state('Chọn thư mục...')
  let input = $state('')
  let theme = $state<Theme>('system')
  let backendReady = $state(false)
  let isStreaming = $state(false)
  let messagesBySession = $state<Record<string, Message[]>>({ welcome: [], workspace: [], crush: [] })

  let activeSession = $derived(sessions.find((session) => session.id === activeSessionId))
  let activeMessages = $derived(messagesBySession[activeSessionId] ?? [])

  $effect(() => {
    if (typeof window === 'undefined') return
    const saved = localStorage.getItem('gotack.theme') as Theme | null
    if (saved === 'light' || saved === 'dark' || saved === 'system') theme = saved
    applyTheme(theme)

    if (hasDesktopBridge()) {
      void callDesktop<boolean>('BackendReady')
        .then((ready) => (backendReady = ready))
        .catch(() => (backendReady = false))
    }
  })

  function applyTheme(value: Theme) {
    if (typeof window === 'undefined') return
    const dark = value === 'dark' || (value === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches)
    document.documentElement.classList.toggle('dark', dark)
    document.documentElement.dataset.theme = dark ? 'dark' : 'light'
    document.documentElement.style.colorScheme = dark ? 'dark' : 'light'
    localStorage.setItem('gotack.theme', value)
  }

  function setTheme(value: Theme) {
    theme = value
    applyTheme(value)
  }

  function newSession() {
    const id = crypto.randomUUID()
    const session: Session = { id, title: 'Hội thoại mới', updatedAt: Date.now() }
    sessions = [session, ...sessions]
    messagesBySession[id] = []
    activeSessionId = id
    input = ''
  }

  function selectSession(id: string) {
    activeSessionId = id
    input = ''
  }

  function renameSession(id: string, title: string) {
    sessions = sessions.map((session) => session.id === id ? { ...session, title, updatedAt: Date.now() } : session)
  }

  function togglePinSession(id: string) {
    sessions = sessions.map((session) => session.id === id ? { ...session, pinned: !session.pinned } : session)
  }

  function deleteSession(id: string) {
    const remaining = sessions.filter((session) => session.id !== id)
    sessions = remaining
    delete messagesBySession[id]
    if (activeSessionId === id) {
      if (remaining.length > 0) activeSessionId = remaining[0].id
      else newSession()
    }
  }

  function setInput(value: string) {
    input = value
  }

  function send() {
    const content = input.trim()
    if (!content || isStreaming) return

    const current = messagesBySession[activeSessionId] ?? []
    messagesBySession[activeSessionId] = [...current, { role: 'user', content }]
    input = ''
    sessions = sessions.map((session) => session.id === activeSessionId ? { ...session, updatedAt: Date.now() } : session)
    isStreaming = true
    sessions = sessions.map((session) => session.id === activeSessionId ? { ...session, streaming: true } : session)

    const response = backendReady
      ? 'UI đã nối được Wails. Luồng Crush streaming/session API sẽ được gắn vào shell này ở bước backend.'
      : 'Đây là UI preview của Gotack. Wails/Crush backend chưa được attach trong phiên frontend này.'

    queueMicrotask(() => {
      const latest = messagesBySession[activeSessionId] ?? []
      messagesBySession[activeSessionId] = [...latest, { role: 'assistant', content: response }]
      isStreaming = false
      sessions = sessions.map((session) => session.id === activeSessionId ? { ...session, streaming: false } : session)
    })
  }

  function pickWorkspace() {
    if (workspace === 'Chọn thư mục...') workspace = 'gotack'
  }
</script>

<div class="app-shell">
  <div class="workspace-frame" style={`--sidebar-col: ${sidebarOpen ? 'var(--mm-sidebar-w)' : '0px'}`}>
    <Sidebar
      {sessions}
      {activeSessionId}
      {workspace}
      onNewSession={newSession}
      onSelectSession={selectSession}
      onCollapse={() => (sidebarOpen = false)}
      onOpenSettings={() => (settingsOpen = true)}
      onRename={renameSession}
      onTogglePin={togglePinSession}
      onDelete={deleteSession}
      onPickWorkspace={pickWorkspace}
    />

    <main class="min-w-0 flex flex-col overflow-hidden bg-mm-bg">
      <ChatArea
        sessionTitle={activeSession?.title ?? 'Gotack'}
        {workspace}
        {sidebarOpen}
        messages={activeMessages}
        {input}
        {backendReady}
        {isStreaming}
        onInput={setInput}
        onSend={send}
        onOpenSidebar={() => (sidebarOpen = true)}
        onOpenSettings={() => (settingsOpen = true)}
        onRenameSession={(title) => renameSession(activeSessionId, title)}
        onPickWorkspace={pickWorkspace}
      />
    </main>
  </div>

  {#if settingsOpen}
    <SettingsModal {theme} onThemeChange={setTheme} onClose={() => (settingsOpen = false)} />
  {/if}

  <Toaster theme={theme === 'system' ? 'system' : theme} position="bottom-right" richColors closeButton />
</div>

<style>
  .app-shell {
    position: relative;
    width: 100%;
    height: 100%;
    min-height: 0;
    overflow: hidden;
    background: var(--tack-app-bg);
  }

  .workspace-frame {
    display: grid;
    grid-template-columns: var(--sidebar-col) minmax(0, 1fr);
    grid-template-rows: minmax(0, 1fr);
    width: 100%;
    height: 100%;
    min-height: 0;
    overflow: hidden;
    transition: grid-template-columns 140ms ease;
  }
</style>
