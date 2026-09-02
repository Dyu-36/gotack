<script lang="ts">
  import { slide } from 'svelte/transition'
  import type { Message } from '../features/conversations/types.svelte'
  import { parseToolDisplay } from '../lib/tool-display'

  type Props = {
    message: Message
    nested?: boolean
  }

  let { message, nested = false }: Props = $props()

  let isRunning = $derived(!message.toolFinished)
  let displayInfo = $derived(parseToolDisplay(message.toolName, message.content, message.toolFinished))

  let expanded = $state(false)
  let copied = $state(false)
  let copiedTimer: ReturnType<typeof setTimeout> | null = null

  function toggle() {
    expanded = !expanded
  }

  async function copyParams() {
    const textToCopy = displayInfo.formattedParams || message.content || ''
    if (!textToCopy) return
    try {
      await navigator.clipboard.writeText(textToCopy)
      copied = true
      if (copiedTimer) clearTimeout(copiedTimer)
      copiedTimer = setTimeout(() => {
        copied = false
      }, 2000)
    } catch (err) {
      console.error('Failed to copy tool parameters:', err)
    }
  }
</script>

<div
  class="tool-card animate-tool-in"
  class:mb-3={!nested}
  class:mr-8={!nested}
  class:sm:mr-20={!nested}
  class:is-nested={nested}
  class:is-running={isRunning}
  class:is-expanded={expanded}
  role="region"
  aria-label={`Công cụ ${displayInfo.detailLabel}`}
>
  <button
    type="button"
    class="tool-header w-full flex items-center justify-between gap-2.5 px-3 py-2 text-left cursor-pointer select-none bg-mm-panel hover:bg-mm-hover transition-colors rounded-lg"
    class:rounded-b-none={expanded && Boolean(displayInfo.formattedParams || message.content)}
    onclick={toggle}
    aria-expanded={expanded}
  >
    <div class="flex items-center gap-2 min-w-0 flex-1">
      <div
        class="tool-icon-wrap shrink-0 flex items-center justify-center w-4 h-4 text-mm-secondary"
        class:tool-icon-pulse={isRunning}
        class:text-mm-accent={isRunning}
      >
        {#if displayInfo.category === 'terminal'}
          <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <polyline points="4 17 10 11 4 5"></polyline>
            <line x1="12" y1="19" x2="20" y2="19"></line>
          </svg>
        {:else if displayInfo.category === 'read'}
          <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path>
            <circle cx="12" cy="12" r="3"></circle>
          </svg>
        {:else if displayInfo.category === 'edit'}
          <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path d="M12 20h9"></path>
            <path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z"></path>
          </svg>
        {:else if displayInfo.category === 'search'}
          <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <circle cx="11" cy="11" r="8"></circle>
            <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
          </svg>
        {:else if displayInfo.category === 'list'}
          <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path>
          </svg>
        {:else}
          <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <circle cx="12" cy="12" r="3"></circle>
            <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"></path>
          </svg>
        {/if}
      </div>

      <div class="flex items-baseline gap-1.5 min-w-0 truncate text-xs">
        <span class="tool-action text-mm-secondary shrink-0 font-normal">
          {displayInfo.actionLabel}
        </span>
        <span
          class="tool-title truncate font-medium text-mm-text"
          class:tool-shimmer={isRunning}
          class:font-mono={displayInfo.isCode}
        >
          {displayInfo.detailLabel}
        </span>
      </div>
    </div>

    <div class="flex items-center gap-2 shrink-0">
      <span
        class="status-pill text-2xs px-1.5 py-0.5 rounded font-medium transition-colors"
        class:bg-amber-500-15={isRunning}
        class:text-amber-600={isRunning}
        class:dark:text-amber-400={isRunning}
        class:bg-emerald-500-15={!isRunning}
        class:text-emerald-600={!isRunning}
        class:dark:text-emerald-400={!isRunning}
      >
        {isRunning ? 'đang chạy' : 'hoàn thành'}
      </span>

      <svg
        class="chevron-icon w-3.5 h-3.5 text-mm-tertiary transition-transform duration-200"
        class:rotate-180={expanded}
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <polyline points="6 9 12 15 18 9"></polyline>
      </svg>
    </div>
  </button>

  {#if expanded && (displayInfo.formattedParams || message.content)}
    <div
      class="tool-body border-t border-mm-border bg-mm-panel/50 p-2.5 relative group"
      transition:slide={{ duration: 180 }}
    >
      <div class="scroll-fade-v max-h-56 overflow-y-auto font-mono text-2xs leading-relaxed text-mm-text">
        {#if displayInfo.isCode}
          <div class="flex items-start gap-1.5 font-semibold text-mm-accent mb-1.5">
            <span class="select-none opacity-60">$</span>
            <span class="break-all">{displayInfo.detailLabel}</span>
          </div>
        {/if}
        <pre class="whitespace-pre-wrap break-words bg-mm-bg dark:bg-[#14161a] p-2 rounded border border-mm-border/60"><code>{displayInfo.formattedParams || message.content}</code></pre>
      </div>

      <button
        type="button"
        class="copy-btn absolute top-3.5 right-3.5 px-2 py-1 rounded text-2xs bg-mm-panel border border-mm-border text-mm-secondary hover:text-mm-text hover:bg-mm-hover transition-all opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 shadow-xs cursor-pointer"
        onclick={copyParams}
        title="Sao chép tham số"
      >
        {#if copied}
          <span class="text-mm-success font-medium">Đã chép</span>
        {:else}
          <span>Sao chép</span>
        {/if}
      </button>
    </div>
  {/if}
</div>

<style>
  .tool-card {
    margin-left: 36px;
    border: 1px solid var(--mm-border);
    border-radius: 9px;
    background: var(--mm-panel);
    transition: border-color 150ms ease, box-shadow 150ms ease;
  }

  .tool-card.is-nested {
    margin-left: 0;
    background: var(--mm-bg);
    border-color: var(--mm-border);
  }

  .tool-card:hover {
    border-color: var(--mm-border-strong);
  }

  .tool-card.is-running {
    border-color: color-mix(in srgb, var(--mm-accent) 40%, var(--mm-border));
    box-shadow: 0 0 0 1px color-mix(in srgb, var(--mm-accent) 20%, transparent);
  }

  .bg-amber-500-15 {
    background-color: rgba(245, 158, 11, 0.12);
  }

  .bg-emerald-500-15 {
    background-color: rgba(16, 185, 129, 0.12);
  }

  @media (prefers-reduced-motion: reduce) {
    .tool-card,
    .chevron-icon {
      transition: none !important;
    }
  }
</style>
