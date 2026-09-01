<script lang="ts">
  import Composer from './Composer.svelte'
  import MessageBubble from './MessageBubble.svelte'
  import type { ChatAttachment, Message, ReasoningEffort } from '../features/conversations/types.svelte'

  type Props = {
    sessionTitle: string
    workspace: string
    sidebarOpen: boolean
    isDark?: boolean
    onToggleTheme?: () => void
    messages: Message[]
    input: string
    attachments?: ChatAttachment[]
    backendReady?: boolean
    isStreaming?: boolean
    modelLabel?: string
    thinkingLabel?: string
    selectedModelId?: string
    selectedProviderId?: string
    selectedThinkingId?: string
    onInput: (value: string) => void
    onSend: () => void
    onAttachFiles?: (files: File[]) => void | Promise<void>
    // Present in the desktop host: routes the paperclip to the native dialog.
    onPickFiles?: () => void | Promise<void>
    onRemoveAttachment?: (id: string) => void
    onStop: () => void
    onOpenSidebar: () => void
    onOpenSettings: () => void
    onRenameSession: (title: string) => void
    onPickWorkspace: () => void
    onSelectModel?: (id: string, label: string, providerId?: string) => void
    onSelectThinking?: (id: ReasoningEffort) => void
  }

  let {
    sessionTitle,
    workspace,
    sidebarOpen,
    isDark = false,
    onToggleTheme,
    messages,
    input,
    attachments = [],
    backendReady = false,
    isStreaming = false,
    modelLabel = 'Model mặc định',
    thinkingLabel = 'Think: Auto',
    selectedModelId = '',
    selectedProviderId = '',
    selectedThinkingId = 'none',
    onInput,
    onSend,
    onAttachFiles = () => {},
    onPickFiles,
    onRemoveAttachment = () => {},
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

  // Tool rows can trail the assistant text, so the streaming cursor has to
  // follow the last real text bubble instead of the last array entry.
  const streamingBubbleId = $derived(
    messages.filter((m) => m.role === 'assistant' && m.kind !== 'tool').at(-1)?.id ?? '',
  )

  const promptCards = [
    {
      key: 'files',
      badgeClass: 'bg-blue-500/10 dark:bg-blue-500/15 border-blue-500/25 dark:border-blue-400/30 text-blue-600 dark:text-blue-400 group-hover:bg-blue-500/20 group-hover:border-blue-500/40',
      title: 'Tìm & xử lý tệp',
      desc: 'Tìm, đọc, đổi tên, sắp xếp hoặc cập nhật tệp trên máy',
      prompt: 'Hãy giúp tôi tìm và xử lý các tệp/thư mục sau trên máy: ',
    },
    {
      key: 'docs',
      badgeClass: 'bg-emerald-500/10 dark:bg-emerald-500/15 border-emerald-500/25 dark:border-emerald-400/30 text-emerald-600 dark:text-emerald-400 group-hover:bg-emerald-500/20 group-hover:border-emerald-500/40',
      title: 'Soạn tài liệu',
      desc: 'Tạo hoặc chỉnh sửa nội dung và tài liệu làm việc',
      prompt: 'Hãy giúp tôi soạn hoặc chỉnh sửa tài liệu sau: ',
    },
    {
      key: 'analyze',
      badgeClass: 'bg-violet-500/10 dark:bg-violet-500/15 border-violet-500/25 dark:border-violet-400/30 text-violet-600 dark:text-violet-400 group-hover:bg-violet-500/20 group-hover:border-violet-500/40',
      title: 'Phân tích thông tin',
      desc: 'Đọc dữ liệu, tài liệu hoặc tệp và rút ra kết luận',
      prompt: 'Hãy phân tích nội dung/dữ liệu sau và đưa ra kết luận hữu ích: ',
    },
    {
      key: 'task',
      badgeClass: 'bg-amber-500/10 dark:bg-amber-500/15 border-amber-500/25 dark:border-amber-400/30 text-amber-600 dark:text-amber-400 group-hover:bg-amber-500/20 group-hover:border-amber-500/40',
      title: 'Thực hiện tác vụ',
      desc: 'Tự tìm cách và dùng công cụ trên máy để hoàn thành việc',
      prompt: 'Hãy thực hiện tác vụ sau trên máy của tôi và hoàn thành đến cùng: ',
    },
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

    <div class="flex items-center gap-1">
      {#if onToggleTheme}
        <button type="button" class="p-1.5 rounded hover:bg-mm-hover text-mm-secondary hover:text-mm-text transition-colors" title={isDark ? "Chuyển sang giao diện sáng" : "Chuyển sang giao diện tối"} aria-label="Đổi giao diện" onclick={onToggleTheme}>
          {#if isDark}
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" /></svg>
          {:else}
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" /></svg>
          {/if}
        </button>
      {/if}

      <button type="button" class="p-1.5 rounded hover:bg-mm-hover text-mm-secondary hover:text-mm-text transition-colors" title="Cài đặt" aria-label="Mở cài đặt" onclick={onOpenSettings}>
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" /><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" /></svg>
      </button>
    </div>
  </div>

  {#if messages.length === 0}
    <div class="flex-1 flex flex-col items-center justify-center px-6 py-6 overflow-y-auto min-h-0">
      <div class="w-full max-w-3xl mx-auto flex flex-col items-center py-4">
        <div class="flex flex-col items-center text-center mb-8">
          <div class="w-16 h-16 rounded-2xl bg-mm-panel border border-mm-border flex items-center justify-center shadow-panel mb-4 p-2.5"><img src="/tack.png" alt="Tack Logo" class="w-full h-full object-contain" /></div>
          <h2 class="hero-title font-bold tracking-tight text-mm-text">Tack AI Assistant</h2>
          <p class="text-sm text-mm-secondary mt-2 max-w-lg">Làm việc với tệp, tài liệu và công cụ trên toàn bộ máy. Chọn thư mục chỉ để đặt ngữ cảnh mặc định.</p>
        </div>
        <div class="w-full grid grid-cols-1 sm:grid-cols-2 gap-3.5">
          {#each promptCards as card (card.key)}
            <button
              type="button"
              class="group relative flex items-start gap-3.5 p-4 rounded-xl bg-mm-panel hover:bg-mm-hover border border-mm-border/80 hover:border-mm-border-strong text-left transition-all duration-150 cursor-pointer shadow-sm hover:shadow-panel"
              onclick={() => { onInput(input.trim() ? `${input.trimEnd()}\n${card.prompt}` : card.prompt); composer?.focus() }}
            >
              <div class="w-10 h-10 rounded-xl flex items-center justify-center border shrink-0 transition-transform duration-150 group-hover:scale-105 {card.badgeClass}" aria-hidden="true">
                {#if card.key === 'files'}
                  <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M4 20h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.93a2 2 0 0 1-1.66-.9l-.82-1.2A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13c0 1.1.9 2 2 2Z" />
                    <circle cx="12" cy="13" r="2.25" />
                    <path d="m13.75 14.75 2.25 2.25" />
                  </svg>
                {:else if card.key === 'docs'}
                  <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z" />
                    <polyline points="14 2 14 8 20 8" />
                    <line x1="8" y1="13" x2="16" y2="13" />
                    <line x1="8" y1="17" x2="12" y2="17" />
                  </svg>
                {:else if card.key === 'analyze'}
                  <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M3 3v18h18" />
                    <path d="m19 7-6 6-4-4-4 5" />
                    <circle cx="19" cy="7" r="1.5" fill="currentColor" />
                  </svg>
                {:else if card.key === 'task'}
                  <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M13 2 3 14h9l-1 8 10-12h-9l1-8z" />
                  </svg>
                {/if}
              </div>
              <div class="flex-1 min-w-0 pr-2">
                <div class="text-sm font-semibold text-mm-text group-hover:text-mm-accent transition-colors flex items-center gap-1.5">
                  <span>{card.title}</span>
                </div>
                <div class="text-xs text-mm-secondary mt-1 leading-relaxed">{card.desc}</div>
              </div>
              <div class="opacity-0 group-hover:opacity-100 -translate-x-1 group-hover:translate-x-0 transition-all duration-150 text-mm-tertiary group-hover:text-mm-secondary self-center shrink-0" aria-hidden="true">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                </svg>
              </div>
            </button>
          {/each}
        </div>
      </div>
    </div>
  {:else}
    <div bind:this={scroller} onscroll={handleScroll} class="messages relative flex-1 overflow-y-auto scroll-stable min-h-0" role="log" aria-label="Nội dung hội thoại" aria-live="polite" aria-relevant="additions text" aria-busy={isStreaming}>
      <div class="max-w-5xl mx-auto px-4 pt-6 pb-6">
        {#each messages as message, idx (message.id)}
          {#if message.kind === 'tool'}
            <div class="tool-row mb-4 mr-8 sm:mr-20" aria-label={`Tool ${message.toolName ?? ''}`}>
              <span class:done={message.toolFinished} class="tool-dot"></span>
              <span class="font-mono text-xs text-mm-text font-medium">{message.toolName ?? 'tool'}</span>
              <span class="text-2xs text-mm-tertiary">{message.toolFinished ? 'hoàn thành' : 'đang chạy'}</span>
              {#if message.content}<span class="truncate text-2xs text-mm-secondary max-w-sm">{message.content}</span>{/if}
            </div>
          {:else}
            <MessageBubble
              {message}
              isStreaming={isStreaming && message.role === 'assistant' && message.id === streamingBubbleId}
            />
          {/if}
        {/each}
        {#if isStreaming && messages.length > 0 && messages.at(-1)?.role === 'user'}
          <div class="flex items-start gap-3 mb-5 animate-fade-in pr-8 sm:pr-20">
            <div class="w-6 h-6 flex-shrink-0 rounded-md bg-mm-panel border border-mm-border flex items-center justify-center p-0.5 mt-0.5 overflow-hidden shadow-xs">
              <img src="/tack.png" alt="Tack" class="w-full h-full object-contain" />
            </div>
            <div class="flex items-center gap-1.5 py-2" aria-label="Đang trả lời">
              <span class="thinking-dot"></span>
              <span class="thinking-dot"></span>
              <span class="thinking-dot"></span>
            </div>
          </div>
        {/if}
      </div>
      {#if !pinned}
        <button type="button" class="jump-latest" onclick={jumpToLatest}>Xuống tin mới nhất</button>
      {/if}
    </div>
  {/if}

  <div class="shrink-0 pt-2">
    <Composer bind:this={composer} value={input} {attachments} conversationStarted={messages.length > 0} onInput={onInput} onSend={onSend} {onAttachFiles} {onPickFiles} {onRemoveAttachment} onStop={onStop} ready={backendReady} {isStreaming} {modelLabel} {thinkingLabel} {selectedModelId} {selectedProviderId} {selectedThinkingId} {onSelectModel} {onSelectThinking} {onOpenSettings} />
  </div>
</div>

<style>
  .hero-title { font-size: 24px; line-height: 32px; }
  .status-dot { width: 6px; height: 6px; border-radius: 999px; background: var(--mm-tertiary); }
  .status-dot.online { background: var(--mm-success, #448361); }
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
