<script lang="ts">
  type Session = {
    id: string
    title: string
    updatedAt: number
    streaming?: boolean
  }

  type Props = {
    sessions: Session[]
    activeSessionId: string
    workspace: string
    onNewSession: () => void
    onSelectSession: (id: string) => void
    onCollapse: () => void
    onOpenSettings: () => void
    onRename: (id: string, title: string) => void
    onDelete: (id: string) => void
    onPickWorkspace: () => void
  }

  let {
    sessions,
    activeSessionId,
    workspace,
    onNewSession,
    onSelectSession,
    onCollapse,
    onOpenSettings,
    onRename,
    onDelete,
    onPickWorkspace,
  }: Props = $props()

  let searchQuery = $state('')
  let editingId = $state<string | null>(null)
  let editingTitle = $state('')

  let filteredSessions = $derived(
    sessions.filter((session) => session.title.toLowerCase().includes(searchQuery.trim().toLowerCase())),
  )

  function formatDate(ts: number): string {
    const date = new Date(ts)
    const diff = Date.now() - date.getTime()
    if (diff < 86_400_000) return date.toLocaleTimeString('vi', { hour: '2-digit', minute: '2-digit' })
    if (diff < 604_800_000) return date.toLocaleDateString('vi', { weekday: 'short' })
    return date.toLocaleDateString('vi', { day: '2-digit', month: '2-digit' })
  }

  function startRename(session: Session) {
    editingId = session.id
    editingTitle = session.title
    queueMicrotask(() => document.getElementById(`rename-${session.id}`)?.focus())
  }

  function commitRename() {
    if (!editingId) return
    const id = editingId
    const title = editingTitle.trim()
    editingId = null
    editingTitle = ''
    if (title) onRename(id, title)
  }

  function handleRenameKeydown(event: KeyboardEvent) {
    if (event.key === 'Enter') commitRename()
    if (event.key === 'Escape') {
      editingId = null
      editingTitle = ''
    }
  }
</script>

<aside class="w-sidebar h-full flex flex-col bg-mm-sidebar border-r border-mm-border overflow-hidden" aria-label="Danh sách hội thoại">
  <div class="flex items-center justify-between px-3 pt-3 pb-1" data-wails-drag-region>
    <div class="flex items-center gap-2">
      <img src="/tack.png" alt="Tack Logo" class="w-6 h-6 object-contain" />
      <span class="text-sm font-semibold text-mm-text">Tack</span>
    </div>
    <button type="button" class="p-1 rounded hover:bg-mm-hover" title="Thu gọn sidebar" aria-label="Thu gọn sidebar" onclick={onCollapse}>
      <svg class="w-4 h-4 text-mm-secondary" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 19l-7-7 7-7M18 19l-7-7 7-7" /></svg>
    </button>
  </div>

  <div class="px-2.5 pt-2 pb-1">
    <button type="button" class="mm-nav-item group w-full justify-center gap-2.5 h-new-chat text-md font-medium" onclick={onNewSession}>
      <svg class="w-5 h-5 text-mm-secondary group-hover:text-mm-text" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" /></svg>
      <span>Hội thoại mới</span>
    </button>
  </div>

  <div class="px-2.5 pt-1 pb-1">
    <button type="button" class="w-full flex items-center justify-between gap-2 px-2.5 py-1.5 rounded-md bg-mm-panel hover:bg-mm-hover border border-mm-border/60 text-left transition-colors group" title={workspace} onclick={onPickWorkspace}>
      <div class="flex items-center gap-2 min-w-0 flex-1">
        <svg class="w-3.5 h-3.5 text-mm-secondary shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" /></svg>
        <div class="flex-1 min-w-0"><div class="text-3xs text-mm-tertiary uppercase tracking-wider font-semibold">Workspace</div><div class="text-xs text-mm-text truncate font-mono">{workspace}</div></div>
      </div>
      <svg class="w-3 h-3 text-mm-tertiary group-hover:text-mm-secondary shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" /></svg>
    </button>
  </div>

  <div class="px-2.5 pt-1 pb-2">
    <div class="relative">
      <svg class="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-mm-secondary" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" /></svg>
      <input type="text" bind:value={searchQuery} placeholder="Tìm kiếm..." aria-label="Tìm hội thoại" class="w-full h-8 pl-8 pr-2 text-sm rounded-md bg-mm-panel border border-transparent focus:border-mm-border focus:bg-mm-bg text-mm-text placeholder:text-mm-tertiary" />
    </div>
  </div>

  <div class="px-3"><div class="border-t border-mm-border"></div></div>
  <div class="px-3 pt-2 pb-1"><span class="text-xs font-medium text-mm-tertiary uppercase tracking-wider">Gần đây</span></div>

  <div class="flex-1 overflow-y-auto scroll-stable px-1.5 pb-2">
    {#if filteredSessions.length === 0}
      <div class="px-3 py-4 text-xs text-mm-tertiary">Không có hội thoại phù hợp.</div>
    {/if}
    {#each filteredSessions as session (session.id)}
      <div class="relative group session-row">
        {#if editingId === session.id}
          <div class="flex items-center gap-1 px-2 h-item mx-1">
            <input id={`rename-${session.id}`} class="input-inline flex-1" bind:value={editingTitle} onkeydown={handleRenameKeydown} onblur={commitRename} aria-label="Tên hội thoại" />
          </div>
        {:else}
          <button type="button" class:mm-nav-active={activeSessionId === session.id} class="mm-nav-item w-full text-left h-item" onclick={() => onSelectSession(session.id)}>
            <svg class="w-3.5 h-3.5 text-mm-secondary shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" /></svg>
            <div class="flex-1 min-w-0 pr-1"><div class="truncate text-sm leading-tight">{session.title}</div><div class="text-xs text-mm-tertiary mt-0.5">{formatDate(session.updatedAt)}</div></div>
            {#if session.streaming}<span class="w-1.5 h-1.5 rounded-pill bg-mm-accent shrink-0" title="Đang trả lời"></span>{/if}
          </button>
          <div class="session-actions absolute right-2 top-1/2 -translate-y-1/2 items-center gap-0.5 bg-mm-panel rounded-md">
            <button type="button" class="p-1 hover:bg-mm-hover rounded" title="Đổi tên" onclick={(event) => { event.stopPropagation(); startRename(session) }}>
              <svg class="w-3 h-3 text-mm-secondary" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" /></svg>
            </button>
            <button type="button" class="p-1 hover:bg-red-500/10 rounded" title="Xóa" onclick={(event) => { event.stopPropagation(); onDelete(session.id) }}>
              <svg class="w-3 h-3 text-red-500" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6M4 7h16" /></svg>
            </button>
          </div>
        {/if}
      </div>
    {/each}
  </div>

  <div class="shrink-0 border-t border-mm-border p-2">
    <button type="button" class="mm-nav-item w-full" onclick={onOpenSettings}>
      <svg class="w-4 h-4 text-mm-secondary" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0" /></svg>
      <span>Cài đặt</span>
    </button>
  </div>
</aside>

<style>
  .session-actions { display: none; }
  .session-row:hover .session-actions { display: flex; }
  .input-inline { min-width: 0; height: 30px; padding: 0 8px; border: 1px solid var(--mm-accent); border-radius: 5px; background: var(--mm-bg); color: var(--mm-text); font: inherit; outline: none; }
</style>
