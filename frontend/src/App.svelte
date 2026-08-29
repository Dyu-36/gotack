<script lang="ts">
  import { Toaster } from 'svelte-sonner'
  import { createThemeState } from './app/theme.svelte'
  import Sidebar from './components/Sidebar.svelte'
  import ChatArea from './components/ChatArea.svelte'
  import SettingsModal from './components/SettingsModal.svelte'
  import RequestModals from './components/RequestModals.svelte'
  import ChangedFilesPanel from './components/ChangedFilesPanel.svelte'
  import TerminalPanel from './components/TerminalPanel.svelte'
  import { createLiveConversationState } from './features/conversations/live-conversation-state.svelte'

  const conversations = createLiveConversationState()
  const theme = createThemeState()

  let sidebarOpen = $state(true)
  let settingsOpen = $state(false)
  let rightPanel = $state<'changes' | 'terminal' | null>(null)

  $effect(() => {
    theme.initialize()
    void conversations.init()
    return () => {
      conversations.destroy()
      theme.destroy()
    }
  })

  function pickWorkspace() {
    rightPanel = null
    void conversations.pickWorkspace()
  }
</script>

<div class="app-shell">
  <div class="workspace-frame" style={`--sidebar-col: ${sidebarOpen ? 'var(--mm-sidebar-w)' : '0px'}`}>
    <Sidebar
      sessions={conversations.sessions}
      activeSessionId={conversations.activeId}
      workspace={conversations.workspace}
      onNewSession={() => void conversations.create()}
      onSelectSession={(id) => void conversations.select(id)}
      onCollapse={() => (sidebarOpen = false)}
      onOpenSettings={() => (settingsOpen = true)}
      onRename={(id, title) => void conversations.rename(id, title)}
      onDelete={(id) => void conversations.delete(id)}
      onPickWorkspace={pickWorkspace}
    />

    <main class="min-w-0 flex overflow-hidden bg-mm-bg relative">
      <div class="min-w-0 flex-1 h-full">
        <ChatArea
          sessionTitle={conversations.active?.title ?? 'Tack'}
          workspace={conversations.workspace}
          {sidebarOpen}
          messages={conversations.active?.messages ?? []}
          input={conversations.input}
          backendReady={conversations.backendReady}
          isStreaming={conversations.active?.status === 'streaming'}
          modelLabel={conversations.modelLabel}
          thinkingLabel={conversations.thinkingLabel}
          selectedModelId={conversations.model}
          selectedThinkingId={conversations.thinking}
          onInput={conversations.setInput}
          onSend={() => void conversations.send()}
          onStop={() => void conversations.cancel()}
          onOpenSidebar={() => (sidebarOpen = true)}
          onOpenSettings={() => (settingsOpen = true)}
          onRenameSession={(title) => void conversations.rename(conversations.activeId, title)}
          onPickWorkspace={pickWorkspace}
          onSelectModel={(id, label, providerId, type) => conversations.setModel(id, label, providerId, type)}
          onSelectThinking={(value) => conversations.setThinking(value)}
        />
      </div>

      <div class="workspace-tools" aria-label="Coding tools">
        <button type="button" class:active={rightPanel === 'changes'} onclick={() => (rightPanel = rightPanel === 'changes' ? null : 'changes')} title="Changed Files">Files</button>
        <button type="button" class:active={rightPanel === 'terminal'} onclick={() => (rightPanel = rightPanel === 'terminal' ? null : 'terminal')} title="Terminal">Terminal</button>
      </div>

      {#if rightPanel === 'changes' && conversations.activeId}
        <ChangedFilesPanel sessionId={conversations.activeId} onClose={() => (rightPanel = null)} />
      {:else if rightPanel === 'terminal' && conversations.workspace !== 'Chọn thư mục...'}
        <TerminalPanel cwd={conversations.workspace} onClose={() => (rightPanel = null)} />
      {/if}
    </main>
  </div>

  {#if conversations.error}
    <div class="status-error" role="status">{conversations.error}</div>
  {/if}

  <RequestModals
    permission={conversations.permission}
    question={conversations.question}
    onPermission={(decision) => void conversations.answerPermission(decision)}
    onQuestion={(answers) => void conversations.answerQuestion(answers)}
  />

  {#if settingsOpen}
    <SettingsModal
      theme={theme.value}
      provider={conversations.provider}
      model={conversations.model}
      smallModel={conversations.smallModel}
      thinking={conversations.thinking}
      apiKey={conversations.apiKey}
      customUrl={conversations.customUrl}
      autostartEngine={conversations.autostartEngine}
      onThemeChange={theme.set}
      onSaveSettings={(settings) => {
        void conversations.saveSettings(settings)
        theme.set(settings.theme)
      }}
      onClose={() => (settingsOpen = false)}
    />
  {/if}

  <Toaster theme={theme.value === 'system' ? 'system' : theme.value} position="bottom-right" richColors closeButton />
</div>

<style>
  .app-shell { position: relative; width: 100%; height: 100%; min-height: 0; overflow: hidden; background: var(--tack-app-bg); }
  .workspace-frame { display: grid; grid-template-columns: var(--sidebar-col) minmax(0, 1fr); grid-template-rows: minmax(0, 1fr); width: 100%; height: 100%; min-height: 0; overflow: hidden; transition: grid-template-columns 140ms ease; }
  .status-error { position: absolute; left: 50%; bottom: 18px; transform: translateX(-50%); max-width: min(680px, 90vw); padding: 9px 12px; border: 1px solid var(--mm-border); border-radius: 8px; background: var(--mm-bg); box-shadow: 0 8px 30px rgb(0 0 0 / 14%); font-size: 12px; z-index: 20; }
  .workspace-tools { position: absolute; top: 48px; right: 10px; z-index: 15; display: flex; gap: 4px; padding: 3px; border: 1px solid var(--mm-border); border-radius: 7px; background: color-mix(in srgb, var(--mm-bg) 92%, transparent); box-shadow: 0 5px 16px rgb(0 0 0 / 8%); }
  .workspace-tools button { padding: 4px 7px; border-radius: 5px; font-size: 10px; color: var(--mm-secondary); }
  .workspace-tools button:hover, .workspace-tools button.active { background: var(--mm-hover); color: var(--mm-text); }
</style>
