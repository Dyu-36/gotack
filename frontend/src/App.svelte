<script lang="ts">
  import { Toaster } from 'svelte-sonner'
  import { createThemeState } from './app/theme.svelte'
  import Sidebar from './components/Sidebar.svelte'
  import ChatArea from './components/ChatArea.svelte'
  import ProviderUsageBadge from './components/ProviderUsageBadge.svelte'
  import SettingsModal from './components/SettingsModal.svelte'
  import RequestModals from './components/RequestModals.svelte'
  import { createLiveConversationState } from './features/conversations/live-conversation-state.svelte'

  import { onMount } from 'svelte'

  const conversations = createLiveConversationState()
  const theme = createThemeState()

  let sidebarOpen = $state(true)
  let settingsOpen = $state(false)

  onMount(() => {
    theme.initialize()
    void conversations.init()
    return () => {
      conversations.destroy()
      theme.destroy()
    }
  })

  function pickWorkspace() {
    void conversations.pickWorkspace()
  }
</script>

<div class="app-shell">
  <div class="workspace-frame" style={`--sidebar-col: ${sidebarOpen ? 'var(--mm-sidebar-w)' : '0px'}`}>
    <Sidebar
      sessions={conversations.sessions}
      activeSessionId={conversations.activeId}
      workspace={conversations.workspace}
      isDark={theme.isDark}
      onToggleTheme={theme.toggle}
      onNewSession={() => void conversations.create()}
      onSelectSession={(id) => void conversations.select(id)}
      onCollapse={() => (sidebarOpen = false)}
      onOpenSettings={() => (settingsOpen = true)}
      onRename={(id, title) => void conversations.rename(id, title)}
      onDelete={(id) => void conversations.delete(id)}
      onPickWorkspace={pickWorkspace}
    />

    <main class="min-w-0 flex overflow-hidden bg-mm-bg relative">
      <div class="min-w-0 flex-1 h-full relative">
        <ChatArea
          sessionTitle={conversations.active?.title ?? 'Tack'}
          workspace={conversations.workspace}
          {sidebarOpen}
          isDark={theme.isDark}
          onToggleTheme={theme.toggle}
          messages={conversations.active?.messages ?? []}
          input={conversations.input}
          attachments={conversations.attachments}
          backendReady={conversations.backendReady}
          isStreaming={conversations.active?.status === 'streaming'}
          modelLabel={conversations.modelLabel}
          thinkingLabel={conversations.thinkingLabel}
          selectedModelId={conversations.model}
          selectedProviderId={conversations.provider}
          selectedThinkingId={conversations.thinking}
          onInput={conversations.setInput}
          onSend={() => void conversations.send()}
          onAttachFiles={(files) => conversations.attachFiles(files)}
          onPickFiles={conversations.hasFilePicker ? () => conversations.pickFiles() : undefined}
          onRemoveAttachment={(id) => conversations.removeAttachment(id)}
          onStop={() => void conversations.cancel()}
          onOpenSidebar={() => (sidebarOpen = true)}
          onOpenSettings={() => (settingsOpen = true)}
          onRenameSession={(title) => void conversations.rename(conversations.activeId, title)}
          onPickWorkspace={pickWorkspace}
          onSelectModel={(id, label, providerId) => conversations.setModel(id, label, providerId)}
          onSelectThinking={(value) => conversations.setThinking(value)}
        />
        <div class="provider-usage-slot">
          <ProviderUsageBadge
            providerId={conversations.provider}
            ready={conversations.backendReady}
          />
        </div>
      </div>
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
    secondsLeft={conversations.permissionSecondsLeft}
    expired={conversations.permissionExpired}
  />

  {#if settingsOpen}
    <SettingsModal
      theme={theme.value}
      provider={conversations.provider}
      model={conversations.model}
      thinking={conversations.thinking}
      customUrl={conversations.customUrl}
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
  .provider-usage-slot { position: absolute; right: 24px; bottom: 78px; z-index: 14; max-width: calc(100% - 48px); }
  .status-error { position: absolute; left: 50%; bottom: 18px; transform: translateX(-50%); max-width: min(680px, 90vw); padding: 9px 12px; border: 1px solid var(--mm-border); border-radius: 8px; background: var(--mm-bg); box-shadow: 0 8px 30px rgb(0 0 0 / 14%); font-size: 12px; z-index: 20; }

  @media (max-width: 720px) {
    .provider-usage-slot { right: 16px; bottom: 74px; max-width: calc(100% - 32px); }
  }
</style>
