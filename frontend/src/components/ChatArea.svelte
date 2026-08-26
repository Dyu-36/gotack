<script lang="ts">
  import Composer from './Composer.svelte'

  type Message = {
    role: 'user' | 'assistant'
    content: string
  }

  type Props = {
    sessionTitle: string
    workspace: string
    sidebarOpen: boolean
    messages: Message[]
    input: string
    backendReady?: boolean
    isStreaming?: boolean
    onInput: (value: string) => void
    onSend: () => void
    onOpenSidebar: () => void
    onOpenSettings: () => void
    onRenameSession: (title: string) => void
    onPickWorkspace: () => void
  }

  let {
    sessionTitle,
    workspace,
    sidebarOpen,
    messages,
    input,
    backendReady = false,
    isStreaming = false,
    onInput,
    onSend,
    onOpenSidebar,
    onOpenSettings,
    onRenameSession,
    onPickWorkspace,
  }: Props = $props()

  let isRenaming = $state(false)
  let renameValue = $state('')

  const promptCards = [
    {
      key: 'create',
      icon: '📅',
      title: 'Tạo TKB thông minh',
      desc: 'Tự động sắp xếp lịch học, tối ưu thời gian trống',
      prompt: 'Hãy giúp tôi tạo thời khóa biểu cho tuần này với các môn học: ',
    },
    {
      key: 'adjust',
      icon: '🔄',
      title: 'Điều chỉnh lịch học',
      desc: 'Đổi tiết học, dời lịch hoặc tránh xung đột giờ',
      prompt: 'Tôi muốn điều chỉnh lại thời khóa biểu để: ',
    },
    {
      key: 'analyze',
      icon: '📊',
      title: 'Phân tích & Tối ưu',
      desc: 'Đánh giá phân bổ thời gian và tải học tập',
      prompt: 'Hãy phân tích và đánh giá thời khóa biểu hiện tại của tôi: ',
    },
    {
      key: 'suggest',
      icon: '💡',
      title: 'Gợi ý môn học',
      desc: 'Tư vấn đăng ký tín chỉ và kế hoạch học tập',
      prompt: 'Gợi ý cho tôi các môn học phù hợp trong học kỳ tới dựa trên: ',
    },
  ]

  function startRename() {
    renameValue = sessionTitle
    isRenaming = true
    queueMicrotask(() => document.getElementById('chat-title-rename')?.focus())
  }

  function commitRename() {
    const next = renameValue.trim()
    if (next) onRenameSession(next)
    isRenaming = false
  }

  function handleRenameKeydown(event: KeyboardEvent) {
    if (event.key === 'Enter') commitRename()
    if (event.key === 'Escape') isRenaming = false
  }
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
        <input id="chat-title-rename" class="input-inline w-full max-w-xs font-medium" bind:value={renameValue} onkeydown={handleRenameKeydown} onblur={commitRename} aria-label="Tên hội thoại" />
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

      <span class:online={backendReady} class="status-dot" title={backendReady ? 'Wails backend connected' : 'Frontend preview'}></span>
    </div>

    <button type="button" class="p-1.5 rounded hover:bg-mm-hover" title="Cài đặt" aria-label="Mở cài đặt" onclick={onOpenSettings}>
      <svg class="w-4 h-4 text-mm-secondary" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" /><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" /></svg>
    </button>
  </div>

  {#if messages.length === 0}
    <div class="flex-1 flex flex-col items-center justify-center px-6 py-6 overflow-y-auto min-h-0">
      <div class="w-full max-w-3xl mx-auto flex flex-col items-center py-4">
        <div class="flex flex-col items-center text-center mb-8">
          <div class="w-16 h-16 rounded-2xl bg-mm-panel border border-mm-border flex items-center justify-center shadow-panel mb-4 p-2.5">
            <img src="/tack.png" alt="Tack Logo" class="w-full h-full object-contain" />
          </div>
          <h2 class="hero-title font-bold tracking-tight text-mm-text mb-2.5">Tack AI Assistant</h2>
          <p class="text-base text-mm-secondary max-w-lg text-center leading-relaxed">Trợ lý AI desktop chạy trên Crush, với giao diện kế thừa từ Stack</p>
        </div>

        <div class="w-full grid grid-cols-1 sm:grid-cols-2 gap-3.5">
          {#each promptCards as card (card.key)}
            <button type="button" class="group relative flex items-start gap-3.5 p-4 rounded-xl bg-mm-panel hover:bg-mm-hover border border-mm-border/70 hover:border-mm-border-strong text-left transition-all duration-150 cursor-pointer shadow-sm hover:shadow-panel" onclick={() => onInput(card.prompt)}>
              <span class="text-xl p-2 rounded-lg bg-mm-bg border border-mm-border/50 shrink-0 leading-none" aria-hidden="true">{card.icon}</span>
              <div class="flex-1 min-w-0 pr-4">
                <div class="text-sm font-semibold text-mm-text group-hover:text-mm-accent transition-colors">{card.title}</div>
                <div class="text-xs text-mm-secondary mt-1 leading-relaxed">{card.desc}</div>
              </div>
              <svg class="w-4 h-4 text-mm-tertiary group-hover:text-mm-text group-hover:translate-x-0.5 transition-all shrink-0 self-center absolute right-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" /></svg>
            </button>
          {/each}
        </div>
      </div>
    </div>
  {:else}
    <div class="relative flex-1 overflow-y-auto scroll-stable min-h-0" role="log" aria-label="Nội dung hội thoại">
      <div class="max-w-3xl mx-auto px-4 pt-6 pb-6 space-y-6">
        {#each messages as message}
          <article class:user-row={message.role === 'user'} class="message-row">
            {#if message.role === 'assistant'}
              <div class="assistant-mark overflow-hidden p-0.5">
                <img src="/tack.png" alt="Tack" class="w-full h-full object-contain" />
              </div>
            {/if}
            <div class:user-bubble={message.role === 'user'} class:assistant-copy={message.role === 'assistant'} class="message-copy">
              {message.content}
            </div>
          </article>
        {/each}
        {#if isStreaming}
          <div class="message-row">
            <div class="assistant-mark overflow-hidden p-0.5">
              <img src="/tack.png" alt="Tack" class="w-full h-full object-contain" />
            </div>
            <div class="flex items-center gap-1.5 py-2" aria-label="Đang trả lời"><span class="thinking-dot"></span><span class="thinking-dot"></span><span class="thinking-dot"></span></div>
          </div>
        {/if}
      </div>
    </div>
  {/if}

  <div class="shrink-0 pt-2">
    <Composer value={input} onInput={onInput} onSend={onSend} {isStreaming} />
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
  .input-inline { min-width: 0; height: 30px; padding: 0 8px; border: 1px solid var(--mm-accent); border-radius: 5px; background: var(--mm-bg); color: var(--mm-text); font: inherit; outline: none; }
</style>
