<script lang="ts">
  import { desktop, events, on, type ChangedFileInfo } from '../platform/desktop'

  type Props = { sessionId: string; onClose: () => void }
  let { sessionId, onClose }: Props = $props()

  let files = $state<ChangedFileInfo[]>([])
  let selectedPath = $state('')
  let diff = $state('')
  let loading = $state(false)
  let error = $state('')

  function formatBytes(size: number): string {
    if (size < 1024) return `${size} B`
    if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`
    return `${(size / 1024 / 1024).toFixed(1)} MB`
  }

  async function refresh() {
    if (!sessionId) {
      files = []
      return
    }
    try {
      files = await desktop.changedFiles(sessionId)
      error = ''
      if (selectedPath && !files.some((file) => file.path === selectedPath)) {
        selectedPath = ''
        diff = ''
      }
    } catch (cause) {
      error = cause instanceof Error ? cause.message : String(cause)
    }
  }

  async function showDiff(path: string) {
    selectedPath = path
    loading = true
    try {
      diff = await desktop.fileDiff(sessionId, path)
      error = ''
    } catch (cause) {
      diff = ''
      error = cause instanceof Error ? cause.message : String(cause)
    } finally {
      loading = false
    }
  }

  $effect(() => {
    const active = sessionId
    void refresh()
    const off = on<{ session_id: string; path: string }>(events.changesUpdated, (event) => {
      if (event.session_id === active) void refresh()
    })
    return off
  })
</script>

<aside class="h-full w-[min(46vw,640px)] min-w-80 border-l border-mm-border bg-mm-bg flex flex-col" aria-label="Changed files">
  <header class="h-11 px-3 flex items-center justify-between border-b border-mm-border shrink-0">
    <div>
      <div class="text-sm font-semibold text-mm-text">Changed Files</div>
      <div class="text-2xs text-mm-tertiary">{files.length} file{files.length === 1 ? '' : 's'} in session</div>
    </div>
    <button type="button" class="btn-notion px-2 py-1 text-xs" onclick={onClose}>Đóng</button>
  </header>

  {#if error}
    <div class="px-3 py-2 text-xs text-red-500 border-b border-mm-border">{error}</div>
  {/if}

  <div class="flex flex-1 min-h-0">
    <div class="w-52 shrink-0 border-r border-mm-border overflow-y-auto p-1.5">
      {#if files.length === 0}
        <div class="px-2 py-4 text-xs text-mm-tertiary">Chưa có file thay đổi.</div>
      {/if}
      {#each files as file (file.path)}
        <button type="button" class:active={selectedPath === file.path} class="file-row" title={file.path} onclick={() => void showDiff(file.path)}>
          <span class="truncate">{file.path}</span>
          <span class="text-3xs text-mm-tertiary shrink-0">{formatBytes(file.size)}</span>
        </button>
      {/each}
    </div>

    <div class="flex-1 min-w-0 overflow-auto bg-mm-panel/20">
      {#if loading}
        <div class="p-4 text-xs text-mm-secondary">Đang tạo diff…</div>
      {:else if selectedPath}
        {#if diff.includes('... (truncated)')}
          <div class="sticky top-0 z-10 px-3 py-1.5 text-xs border-b border-mm-border bg-mm-bg text-mm-secondary">
            Diff lớn đã được giới hạn ở backend để giữ UI responsive.
          </div>
        {/if}
        <pre class="diff-view" aria-label={`Diff ${selectedPath}`}>{diff || 'Không có thay đổi nội dung để hiển thị.'}</pre>
      {:else}
        <div class="p-4 text-xs text-mm-tertiary">Chọn một file để xem unified diff.</div>
      {/if}
    </div>
  </div>
</aside>

<style>
  .file-row { width: 100%; min-height: 32px; display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 5px 7px; border-radius: 6px; color: var(--mm-secondary); font-size: 12px; text-align: left; }
  .file-row:hover, .file-row.active { background: var(--mm-hover); color: var(--mm-text); }
  .diff-view { margin: 0; padding: 12px; min-width: max-content; font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; font-size: 11px; line-height: 1.55; color: var(--mm-text); white-space: pre; }
</style>
