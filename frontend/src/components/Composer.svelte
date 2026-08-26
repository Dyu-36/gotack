<script lang="ts">
  type Props = {
    value: string
    modelLabel?: string
    thinkingLabel?: string
    isStreaming?: boolean
    onInput: (value: string) => void
    onSend: () => void
    onStop?: () => void
  }

  let {
    value,
    modelLabel = 'Crush',
    thinkingLabel = 'Think: Auto',
    isStreaming = false,
    onInput,
    onSend,
    onStop = () => {},
  }: Props = $props()

  let textarea = $state<HTMLTextAreaElement>()
  let modelMenuOpen = $state(false)
  let thinkingMenuOpen = $state(false)

  let canSend = $derived(value.trim().length > 0 && !isStreaming)

  function autoResize() {
    if (!textarea) return
    textarea.style.height = 'auto'
    textarea.style.height = Math.min(textarea.scrollHeight, 160) + 'px'
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault()
      if (canSend) onSend()
    }
  }
</script>

<div class="w-full max-w-input-w mx-auto px-4 pb-4">
  <div class="rounded-mm border border-mm-border bg-mm-bg transition-all focus-within:border-mm-accent/60 focus-within:ring-2 focus-within:ring-mm-accent/15 shadow-sm">
    <div class="px-4 pt-3.5 pb-2">
      <textarea
        bind:this={textarea}
        value={value}
        oninput={(event) => { onInput(event.currentTarget.value); autoResize() }}
        onkeydown={handleKeydown}
        placeholder="Nhập tin nhắn... (Enter để gửi, Shift+Enter để xuống dòng)"
        rows="1"
        disabled={isStreaming}
        aria-label="Nội dung tin nhắn"
        class="w-full resize-none bg-transparent text-mm-text text-base leading-relaxed placeholder:text-mm-tertiary overflow-y-auto scroll-stable min-h-6 max-h-[var(--composer-max-h)] disabled:opacity-60 focus:outline-none focus-visible:outline-none focus:ring-0"
      ></textarea>
    </div>

    <div class="flex items-center justify-between px-3 pb-3">
      <div class="flex items-center gap-1">
        <button type="button" class="flex items-center justify-center w-8 h-8 rounded-lg text-mm-secondary hover:text-mm-text hover:bg-mm-hover transition-colors" title="Đính kèm tệp" aria-label="Đính kèm tệp">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" /></svg>
        </button>
      </div>

      <div class="flex items-center gap-1">
        <div class="relative">
          <button type="button" class="mm-model-select flex items-center gap-1.5 px-2 text-xs" onclick={() => { thinkingMenuOpen = !thinkingMenuOpen; modelMenuOpen = false }}>
            <svg class="w-3.5 h-3.5 text-mm-secondary" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" /></svg>
            <span>{thinkingLabel}</span>
            <svg class="w-3 h-3 text-mm-tertiary" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" /></svg>
          </button>
          {#if thinkingMenuOpen}
            <div class="menu-pop absolute bottom-full right-0 mb-2 w-44 p-1.5">
              <button class="menu-item active">Auto</button>
              <button class="menu-item">Low</button>
              <button class="menu-item">Medium</button>
              <button class="menu-item">High</button>
            </div>
          {/if}
        </div>

        <div class="relative">
          <button type="button" class="mm-model-select flex items-center gap-1.5 px-2 text-xs" onclick={() => { modelMenuOpen = !modelMenuOpen; thinkingMenuOpen = false }}>
            <span class="w-2 h-2 rounded-full bg-mm-accent"></span>
            <span class="max-w-28 truncate">{modelLabel}</span>
            <svg class="w-3 h-3 text-mm-tertiary" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" /></svg>
          </button>
          {#if modelMenuOpen}
            <div class="menu-pop absolute bottom-full right-0 mb-2 w-56 p-1.5">
              <div class="px-2 py-1.5 text-2xs uppercase tracking-wider text-mm-tertiary font-semibold">Model</div>
              <button class="menu-item active">Crush default</button>
              <button class="menu-item">Claude</button>
              <button class="menu-item">GPT</button>
              <button class="menu-item">Gemini</button>
            </div>
          {/if}
        </div>

        {#if isStreaming}
          <button type="button" class="mm-send-btn" title="Dừng" aria-label="Dừng" onclick={onStop}>
            <svg class="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 24 24"><rect x="7" y="7" width="10" height="10" rx="1" /></svg>
          </button>
        {:else}
          <button type="button" class="mm-send-btn" title="Gửi" aria-label="Gửi" disabled={!canSend} onclick={onSend}>
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 19V5M5 12l7-7 7 7" /></svg>
          </button>
        {/if}
      </div>
    </div>
  </div>
</div>

<style>
  .menu-pop {
    border: 1px solid var(--mm-border);
    border-radius: 10px;
    background: var(--mm-bg);
    box-shadow: var(--shadow-popup, 0 4px 20px rgb(0 0 0 / 0.2));
    z-index: 30;
  }
  .menu-item {
    width: 100%;
    padding: 7px 9px;
    border: 0;
    border-radius: 6px;
    background: transparent;
    color: var(--mm-text);
    text-align: left;
    font: inherit;
    font-size: 12px;
    cursor: pointer;
  }
  .menu-item:hover, .menu-item.active { background: var(--mm-hover); }
</style>
