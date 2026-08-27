<script lang="ts">
  import { Toaster } from 'svelte-sonner'
  import { createThemeState } from './app/theme.svelte'
  import Sidebar from './components/Sidebar.svelte'
  import ChatArea from './components/ChatArea.svelte'
  import SettingsModal from './components/SettingsModal.svelte'
  import { createConversationState } from './features/conversations/conversation-state.svelte'
  import { desktop } from './platform/desktop'

  const conversations = createConversationState()
  const theme = createThemeState()

  let sidebarOpen = $state(true)
  let settingsOpen = $state(false)
  let workspace = $state('Chọn thư mục...')
  let backendReady = $state(false)

  $effect(() => {
    theme.initialize()
    void conversations.loadSettings()
    void desktop.backendReady().then((ready) => (backendReady = ready)).catch(() => (backendReady = false))
    return theme.destroy
  })

  function pickWorkspace() {
    if (workspace === 'Chọn thư mục...') workspace = 'Tack'
  }
</script>

<div class="app-shell">
  <div class="workspace-frame" style={`--sidebar-col: ${sidebarOpen ? 'var(--mm-sidebar-w)' : '0px'}`}>
    <Sidebar
      sessions={conversations.sessions}
      activeSessionId={conversations.activeId}
      {workspace}
      onNewSession={conversations.create}
      onSelectSession={conversations.select}
      onCollapse={() => (sidebarOpen = false)}
      onOpenSettings={() => (settingsOpen = true)}
      onRename={conversations.rename}
      onTogglePin={conversations.togglePin}
      onDelete={conversations.delete}
      onPickWorkspace={pickWorkspace}
    />

    <main class="min-w-0 flex flex-col overflow-hidden bg-mm-bg">
      <ChatArea
        sessionTitle={conversations.active?.title ?? 'Tack'}
        {workspace}
        {sidebarOpen}
        messages={conversations.active?.messages ?? []}
        input={conversations.input}
        {backendReady}
        isStreaming={conversations.active?.status === 'streaming'}
        modelLabel={conversations.modelLabel}
        thinkingLabel={conversations.thinkingLabel}
        selectedModelId={conversations.model}
        selectedThinkingId={conversations.thinking}
        onInput={conversations.setInput}
        onSend={() => conversations.sendPreviewMessage(backendReady)}
        onOpenSidebar={() => (sidebarOpen = true)}
        onOpenSettings={() => (settingsOpen = true)}
        onRenameSession={(title) => conversations.rename(conversations.activeId, title)}
        onPickWorkspace={pickWorkspace}
        onSelectModel={(id, label, pId, type) => conversations.setModel(id, label, pId, type)}
        onSelectThinking={(t) => conversations.setThinking(t)}
      />
    </main>
  </div>

  {#if settingsOpen}
    <SettingsModal
      theme={theme.value}
      provider={conversations.provider}
      model={conversations.model}
      smallModel={conversations.smallModel}
      thinking={conversations.thinking}
      apiKey={conversations.apiKey}
      customUrl={conversations.customUrl}
      onThemeChange={theme.set}
      onSaveSettings={(s) => {
        void conversations.saveSettings(s)
        theme.set(s.theme)
      }}
      onClose={() => (settingsOpen = false)}
    />
  {/if}

  <Toaster theme={theme.value === 'system' ? 'system' : theme.value} position="bottom-right" richColors closeButton />
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
