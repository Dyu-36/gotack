<script lang="ts">
  import Composer from './Composer.svelte'
  import type { Message, ModelType, ReasoningEffort } from '../features/conversations/types.svelte'

  type Props = {
    sessionTitle: string
    workspace: string
    sidebarOpen: boolean
    messages: Message[]
    input: string
    backendReady?: boolean
    isStreaming?: boolean
    modelLabel?: string
    thinkingLabel?: string
    selectedModelId?: string
    selectedThinkingId?: string
    onInput: (value: string) => void
    onSend: () => void
    onStop: () => void
    onOpenSidebar: () => void
    onOpenSettings: () => void
    onRenameSession: (title: string) => void
    onPickWorkspace: () => void
    onSelectModel?: (id: string, label: string, providerId?: string, type?: ModelType) => void
    onSelectThinking?: (id: ReasoningEffort) => void
  }

  let {
    sessionTitle,
    workspace,
    sidebarOpen,
    messages,
    input,
    backendReady = false,
    isStreaming = false,
    modelLabel = 'Crush model',
    thinkingLabel = 'Think: Auto',
    selectedModelId = '',
    selectedThinkingId = 'none',
    onInput,
    onSend,
    onStop,
    onOpenSidebar,
    onOpenSettings,
    onRenameSession,
    onPickWorkspace,
    onSelectModel = () => {},
    onSelectThinking = () => {},
  }: Props = $props()

  let isRenaming = $state(false)
  let renameValue = $state('')
  let renameInput = $state<HTMLInputElement | null>(null)
  let renameCancelled = false
  let scroller = $state<HTMLElement | null>(null)
  let pinned = $state(true)
  let streamingText = $state('')
  let composer = $state<Composer | null>(null)

  const STICK_THRESHOLD = 48

  const promptCards = [
    { key: 'explain', icon: '⌘', title: 'Giải thích codebase', desc: 'Phân tích kiến trúc, luồng dữ liệu và điểm nóng', prompt: 'Hãy phân tích codebase hiện tại và giải thích kiến trúc, luồng dữ liệu chính và các điểm cần chú ý. ' },
    { key: 'fix', icon: '✓', title: 'Sửa lỗi', desc: 'Đi từ triệu chứng tới root cause và patch', prompt: 'Hãy điều tra lỗi sau, tìm root cause, sửa tối thiểu và chạy kiểm tra liên quan: ' },
    { key: 'refactor', icon: '↻', title: 'Refactor an toàn', desc: 'Cải thiện cấu trúc nhưng giữ nguyên behavior', prompt: 'Hãy refactor phần sau để code rõ hơn, ít coupling hơn và giữ nguyên behavior: ' },
    { key: 'test', icon: '◇', title: 'Bổ sung test', desc: 'Thêm test cho behavior và edge case quan trọng', prompt: 'Hãy bổ sung test cho phần sau, ưu tiên behavior, regression và edge cases: ' },
  ]

  function startRename() {
    renameCancelled = false
    renameValue = sessionTitle
    isRenaming = true
  }

  function commitRename() {
    if (renameCancelled) {
      isRenaming = false
      return
    }
    const next = renameValue.trim()
    if (next && next !== sessionTitle) onRenameSession(next)
    isRenaming = false
  }

  function cancelRename() {
    renameCancelled = true
    isRenaming = false
  }

  function handleRenameKeydown(event: KeyboardEvent) {
    if (event.key === 'Enter') commitRename()
    if (event.key === 'Escape') cancelRename()
  }

  function handleScroll() {
    if (!scroller) return
    const distance = scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight
    pinned = distance <= STICK_THRESHOLD
  }

  function jumpToLatest() {
    pinned = true
    if (!scroller) return
    const reduceMotion =
      typeof window !== 'undefined' &&
      window.matchMedia('(prefers-reduced-motion: reduce)').matches
    scroller.scrollTo({ top: scroller.scrollHeight, behavior: reduceMotion ? 'auto' : 'smooth' })
  }

  $effect(() => {
    if (isRenaming) renameInput?.select()
  })

  $effect(() => {
    if (!pinned || !scroller) return
    // Track the streaming message tail so we auto-stick while content grows.
    const lastContent = messages.at(-1)?.content
    void lastContent
    void messages.length
    scroller.scrollTop = scroller.scrollHeight
  })
</script>

<div class="flex flex-col h-full min-h-0">
  <div class="flex items-center gap-2 px-4 py-2.5 border-b border-mm-border shrink-0" data-wails-drag-region>
    {#if !sidebarOpen}
      <button type="button" class="p-1 rounded hover:bg-mm-hover mr-1" onclick={onOpenSidebar} title="Mở sidebar" aria-label="Mở sidebar">
        <svg class="w-4 h-4 text-mm-secondary" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 5l7 7-7 7M6 5l7 7-7 7" /></svg>
      </button>
    {/if}

    <div class="flex-1 min-w-0 flex items-center gap-2">
      {#if isRenaming}
        <input bind:this={renameInput} class="input-inline w-full max-w-xs font-medium" bind:value={renameValue} onkeydown={handleRenameKeydown} onblur={commitRename} aria-label="Tên hội thoại" />
      {:else}
        <button type="button" class="flex items-center gap-1 text-sm font-medium text-mm-text hover:bg-mm-hover px-2 py-1 rounded-md truncate max-w-xs" onclick={startRename} title="Nhấn để đổi tên">
          <span class="truncate">{sessionTitle}</span>
          <svg class="w-3 h-3 text-mm-secondary shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" /></svg>
        </button>
      {/if}

      <button type="button" class="flex items-center gap-1 px-2 py-0.5 rounded text-xs font-mono bg-mm-panel hover:bg-mm-hover border border-mm-border/50 text-mm-secondary hover:text-mm-text transition-colors max-w-48 truncate cursor-pointer" title={workspace} onclick={onPickWorkspace}>
        <svg class="w-3 h-3 shrink-0 text-mm-tertiary" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" /></svg>
        <span class="truncate">{workspace}</span>
      </button>

      <span class:online={backendReady} class="status-dot" title={backendReady ? 'Wails backend connected' : 'Backend unavailable'}></span>
    </div>

    <button type="button" class="p-1.5 rounded hover:bg-mm-hover" title="Cài đặt" aria-label="Mở cài đặt" onclick={onOpenSettings}>
      <svg class="w-4 h-4 text-mm-secondary" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" /><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" /></svg>
    </button>
  </div>

  {#if messages.length === 0}
    <div class="flex-1 flex flex-col items-center justify-center px-6 py-6 overflow-y-auto min-h-0">
      <div class="w-full max-w-3xl mx-auto flex flex-col items-center py-4">
        <div class="flex flex-col items-center text-center mb-8">
          <div class="w-16 h-16 rounded-2xl bg-mm-panel border border-mm-border flex items-center justify-center shadow-panel mb-4 p-2.5"><img src="/tack.png" alt="Tack Logo" class="w-full h-full object-contain" /></div>
          <h2 class="hero-title font-bold tracking-tight text-mm-text mb-2.5">Tack AI Coding Assistant</h2>
          <p class="text-base text-mm-secondary max-w-lg text-center leading-relaxed">Desktop shell nhẹ cho Crush: session, tools, permission, diff và terminal.</p>
        </div>
        <div class="w-full grid grid-cols-1 sm:grid-cols-2 gap-3.5">
          {#each promptCards as card (card.key)}
            <button type="button" class="group relative flex items-start gap-3.5 p-4 rounded-xl bg-mm-panel hover:bg-mm-hover border border-mm-border/70 hover:border-mm-border-strong text-left transition-all duration-150 cursor-pointer shadow-sm hover:shadow-panel" onclick={() => { onInput(input.trim() ? `${input.trimEnd()}\n${card.prompt}` : card.prompt); composer?.focus() }}>
              <span class="text-xl p-2 rounded-lg bg-mm-bg border border-mm-border/50 shrink-0 leading-none" aria-hidden="true">{card.icon}</span>
              <div class="flex-1 min-w-0 pr-4"><div class="text-sm font-semibold text-mm-text group-hover:text-mm-accent transition-colors">{card.title}</div><div class="text-xs text-mm-secondary mt-1 leading-relaxed">{card.desc}</div></div>
            </button>
          {/each}
        </div>
      </div>
    </div>
  {:else}
    <div bind:this={scroller} onscroll={handleScroll} class="messages relative flex-1 overflow-y-auto scroll-stable min-h-0" role="log" aria-label="Nội dung hội thoại" aria-live="polite" aria-relevant="additions text" aria-busy={isStreaming}>
      <div class="max-w-3xl mx-auto px-4 pt-6 pb-6 space-y-4">
        {#each messages as message (message.id)}
          {#if message.kind === 'tool'}
            <div class="tool-row" aria-label={`Tool ${message.toolName ?? ''}`}>
              <span class:done={message.toolFinished} class="tool-dot"></span>
              <span class="font-mono text-xs">{message.toolName ?? 'tool'}</span>
              <span class="text-2xs text-mm-tertiary">{message.toolFinished ? 'done' : 'running'}</span>
              {#if message.content}<span class="truncate text-2xs text-mm-secondary">{message.content}</span>{/if}
            </div>
          {:else}
            <article class:user-row={message.role === 'user'} class="message-row">
              {#if message.role === 'assistant'}<div class="assistant-mark overflow-hidden p-0.5"><img src="/tack.png" alt="Tack" class="w-full h-full object-contain" /></div>{/if}
              <div class:user-bubble={message.role === 'user'} class:assistant-copy={message.role === 'assistant'} class="message-copy">{message.content}</div>
            </article>
          {/if}
        {/each}
        {#if isStreaming && !streamingText}<div class="message-row"><div class="assistant-mark overflow-hidden p-0.5"><img src="/tack.png" alt="Tack" class="w-full h-full object-contain" /></div><div class="flex items-center gap-1.5 py-2" aria-label="Đang trả lời"><span class="thinking-dot"></span><span class="thinking-dot"></span><span class="thinking-dot"></span></div></div>{/if}
      </div>
      {#if !pinned}
        <button type="button" class="jump-latest" onclick={jumpToLatest}>Xuống tin mới nhất</button>
      {/if}
    </div>
  {/if}

  <div class="shrink-0 pt-2">
    <Composer bind:this={composer} value={input} onInput={onInput} onSend={onSend} onStop={onStop} {isStreaming} {modelLabel} {thinkingLabel} {selectedModelId} {selectedThinkingId} {onSelectModel} {onSelectThinking} {onOpenSettings} />
  </div>
</div>

<style>
  .hero-title { font-size: 24px; line-height: 32px; }
  .status-dot { width: 6px; height: 6px; border-radius: 999px; background: var(--mm-tertiary); }
  .status-dot.online { background: var(--mm-success, #448361); }
  .message-row { display: flex; align-items: flex-start; gap: 10px; }
  .message-row.user-row { justify-content: flex-end; }
  .assistant-mark { width: 26px; height: 26px; flex: 0 0 auto; display: grid; place-items: center; border-radius: 7px; background: var(--mm-panel); border: 1px solid var(--mm-border); color: var(--mm-text); font-size: 11px; font-weight: 750; }
  .message-copy { max-width: min(680px, 86%); font-size: 14px; line-height: 1.65; white-space: pre-wrap; overflow-wrap: anywhere; }
  .assistant-copy { padding-top: 2px; color: var(--mm-text); }
  .user-bubble { padding: 9px 13px; border-radius: 14px; background: var(--mm-panel); border: 1px solid var(--mm-border); color: var(--mm-text); }
  .tool-row { display: flex; align-items: center; gap: 7px; min-width: 0; margin-left: 36px; padding: 6px 9px; border: 1px solid var(--mm-border); border-radius: 7px; background: var(--mm-panel); color: var(--mm-secondary); }
  .tool-dot { width: 7px; height: 7px; border-radius: 999px; background: #c89336; flex: 0 0 auto; }
  .tool-dot.done { background: var(--mm-success, #448361); }
  .input-inline { min-width: 0; height: 30px; padding: 0 8px; border: 1px solid var(--mm-accent); border-radius: 5px; background: var(--mm-bg); color: var(--mm-text); font: inherit; outline: none; }
  .messages { overflow-anchor: none; }
  .jump-latest { position: sticky; bottom: 12px; left: 50%; transform: translateX(-50%); display: inline-flex; align-items: center; gap: 6px; padding: 6px 12px; border-radius: 999px; background: var(--mm-panel); border: 1px solid var(--mm-border); color: var(--mm-text); font-size: 12px; font-weight: 500; box-shadow: 0 6px 16px rgba(0, 0, 0, 0.18); cursor: pointer; transition: background-color 120ms ease, transform 120ms ease; }
  .jump-latest:hover { background: var(--mm-hover); }
  .jump-latest:focus-visible { outline: 2px solid var(--mm-accent); outline-offset: 2px; }

  @media (prefers-reduced-motion: reduce) {
    .messages,
    .scroll-stable { scroll-behavior: auto !important; }
    .thinking-dot,
    .jump-latest { animation-duration: 0ms !important; transition-duration: 0ms !important; }
  }
</style>
