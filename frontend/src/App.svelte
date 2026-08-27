<script lang="ts">
  import { Toaster } from 'svelte-sonner'
  import { createThemeState } from './app/theme.svelte'
  import Sidebar from './components/Sidebar.svelte'
  import ChatArea from './components/ChatArea.svelte'
  import SettingsModal from './components/SettingsModal.svelte'
  import { createLiveConversationState } from './features/conversations/live-conversation-state.svelte'

  const conversations = createLiveConversationState()
  const theme = createThemeState()

  let sidebarOpen = $state(true)
  let settingsOpen = $state(false)
  let questionAnswer = $state('')

  $effect(() => {
    theme.initialize()
    void conversations.init()
    return () => {
      conversations.destroy()
      theme.destroy()
    }
  })
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
      onRename={conversations.rename}
      onTogglePin={conversations.togglePin}
      onDelete={conversations.delete}
      onPickWorkspace={() => void conversations.pickWorkspace()}
    />

    <main class="min-w-0 flex flex-col overflow-hidden bg-mm-bg">
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
        onOpenSidebar={() => (sidebarOpen = true)}
        onOpenSettings={() => (settingsOpen = true)}
        onRenameSession={(title) => conversations.rename(conversations.activeId, title)}
        onPickWorkspace={() => void conversations.pickWorkspace()}
        onSelectModel={(id, label, pId, type) => conversations.setModel(id, label, pId, type)}
        onSelectThinking={(t) => conversations.setThinking(t)}
      />
    </main>
  </div>

  {#if conversations.error}
    <div class="status-error" role="status">{conversations.error}</div>
  {/if}

  {#if conversations.permission}
    <div class="request-backdrop">
      <section class="request-card" aria-label="Yêu cầu quyền">
        <h2>Crush cần quyền thực thi</h2>
        <strong>{conversations.permission.tool_name}</strong>
        <p>{conversations.permission.description || conversations.permission.action}</p>
        {#if conversations.permission.path}<code>{conversations.permission.path}</code>{/if}
        <div class="request-actions">
          <button onclick={() => void conversations.answerPermission('deny')}>Từ chối</button>
          <button onclick={() => void conversations.answerPermission('allow_session')}>Cho phép phiên này</button>
          <button class="primary" onclick={() => void conversations.answerPermission('allow')}>Cho phép</button>
        </div>
      </section>
    </div>
  {/if}

  {#if conversations.question}
    <div class="request-backdrop">
      <section class="request-card" aria-label="Câu hỏi từ Crush">
        <h2>{conversations.question.confirm_title ?? 'Crush cần thêm thông tin'}</h2>
        {#if conversations.question.questions[0]}
          {@const q = conversations.question.questions[0]}
          <p>{q.question}</p>
          {#if q.choices?.length}
            <div class="choice-list">
              {#each q.choices as choice}
                <button onclick={() => void conversations.answerQuestion(choice.id)}>{choice.label}</button>
              {/each}
            </div>
          {:else}
            <input bind:value={questionAnswer} placeholder="Nhập câu trả lời" />
            <div class="request-actions">
              <button class="primary" onclick={() => { const answer = questionAnswer; questionAnswer = ''; void conversations.answerQuestion(answer) }}>Gửi</button>
            </div>
          {/if}
        {/if}
      </section>
    </div>
  {/if}

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
  .app-shell { position: relative; width: 100%; height: 100%; min-height: 0; overflow: hidden; background: var(--tack-app-bg); }
  .workspace-frame { display: grid; grid-template-columns: var(--sidebar-col) minmax(0, 1fr); grid-template-rows: minmax(0, 1fr); width: 100%; height: 100%; min-height: 0; overflow: hidden; transition: grid-template-columns 140ms ease; }
  .status-error { position: absolute; left: 50%; bottom: 18px; transform: translateX(-50%); max-width: min(680px, 90vw); padding: 9px 12px; border: 1px solid var(--mm-border); border-radius: 8px; background: var(--mm-bg); box-shadow: 0 8px 30px rgb(0 0 0 / 14%); font-size: 12px; z-index: 20; }
  .request-backdrop { position: absolute; inset: 0; display: grid; place-items: center; background: rgb(0 0 0 / 34%); z-index: 50; }
  .request-card { width: min(520px, calc(100vw - 32px)); padding: 20px; border-radius: 12px; background: var(--mm-bg); border: 1px solid var(--mm-border); box-shadow: 0 20px 50px rgb(0 0 0 / 22%); }
  .request-card h2 { margin: 0 0 12px; font-size: 18px; }
  .request-card p { margin: 10px 0; color: var(--mm-secondary); }
  .request-card code { display: block; padding: 8px; border-radius: 6px; background: var(--mm-hover); overflow-wrap: anywhere; }
  .request-card input { width: 100%; margin-top: 10px; padding: 9px 10px; border: 1px solid var(--mm-border); border-radius: 7px; background: var(--mm-bg); }
  .request-actions, .choice-list { display: flex; gap: 8px; justify-content: flex-end; flex-wrap: wrap; margin-top: 16px; }
  .request-actions button, .choice-list button { padding: 8px 11px; border-radius: 7px; border: 1px solid var(--mm-border); background: var(--mm-bg); }
  .request-actions .primary { background: var(--mm-inverse-surface, #322f29); color: var(--mm-inverse-text, white); }
</style>
