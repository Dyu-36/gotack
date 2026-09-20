<script lang="ts">
  import { Toaster } from 'svelte-sonner'
  import { createThemeState } from './app/theme.svelte'
  import Sidebar from './components/Sidebar.svelte'
  import ChatArea from './components/ChatArea.svelte'
  import ProviderUsageBadge from './components/ProviderUsageBadge.svelte'
  import SettingsModal from './components/SettingsModal.svelte'
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
      onClone={(id) => void conversations.clone(id)}
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
          onCompactSession={() => void conversations.compact(conversations.activeId)}
          onForkMessage={(messageId) => void conversations.fork(conversations.activeId, messageId)}
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


  {#if conversations.workspaceInfo?.trust_required}
    <div class="trust-overlay" role="presentation">
      <section class="trust-card" role="dialog" aria-modal="true" aria-labelledby="project-trust-title">
        <h2 id="project-trust-title">Project này có tài nguyên động</h2>
        <p>Gotack phát hiện resource có thể thay đổi hành vi agent trong workspace này. Tool và quyền hệ điều hành vẫn giữ nguyên; quyết định này chỉ kiểm soát việc nạp resource của project.</p>
        {#if conversations.workspaceInfo.protected_resources?.length}
          <ul>
            {#each conversations.workspaceInfo.protected_resources as resource (resource)}
              <li><code>{resource}</code></li>
            {/each}
          </ul>
        {/if}
        <div class="trust-actions">
          <button type="button" class="btn-notion" onclick={() => void conversations.setWorkspaceTrust(false)}>Mở không trust</button>
          <button type="button" class="trust-primary" onclick={() => void conversations.setWorkspaceTrust(true)}>Trust project</button>
        </div>
      </section>
    </div>
  {/if}

  {#if settingsOpen}
    <SettingsModal
      theme={theme.value}
      provider={conversations.provider}
      model={conversations.model}
      thinking={conversations.thinking}
      customUrl={conversations.customUrl}
      onThemeChange={theme.set}
      onSaveSettings={async (settings) => {
        const saved = await conversations.saveSettings(settings)
        if (!saved) throw new Error(conversations.error || "Could not save settings")
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
  .provider-usage-slot { position: absolute; top: 54px; right: 24px; z-index: 14; max-width: calc(100% - 48px); }
  .provider-usage-slot :global(.usage-popover) { top: calc(100% + 8px); bottom: auto; }
  .status-error { position: absolute; left: 50%; bottom: 18px; transform: translateX(-50%); max-width: min(680px, 90vw); padding: 9px 12px; border: 1px solid var(--mm-border); border-radius: 8px; background: var(--mm-bg); box-shadow: 0 8px 30px rgb(0 0 0 / 14%); font-size: 12px; z-index: 20; }

  @media (max-width: 720px) {
    .provider-usage-slot { top: 52px; right: 16px; max-width: calc(100% - 32px); }
  }
  .trust-overlay { position: absolute; inset: 0; z-index: 45; display: grid; place-items: center; padding: 20px; background: rgb(0 0 0 / 45%); backdrop-filter: blur(3px); }
  .trust-card { width: min(560px, 92vw); padding: 20px; border: 1px solid var(--mm-border); border-radius: 12px; background: var(--mm-bg); box-shadow: 0 24px 70px rgb(0 0 0 / 28%); color: var(--mm-text); }
  .trust-card h2 { margin: 0 0 8px; font-size: 17px; font-weight: 650; }
  .trust-card p { margin: 0 0 12px; color: var(--mm-secondary); font-size: 13px; line-height: 1.55; }
  .trust-card ul { max-height: 180px; overflow: auto; margin: 0 0 16px; padding: 10px 12px 10px 30px; border: 1px solid var(--mm-border); border-radius: 8px; background: var(--mm-panel); font-size: 12px; }
  .trust-actions { display: flex; justify-content: flex-end; gap: 8px; }
  .trust-primary { padding: 7px 12px; border-radius: 7px; background: var(--mm-accent); color: white; font-size: 12px; font-weight: 600; }
</style>
