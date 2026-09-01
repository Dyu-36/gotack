<script lang="ts">
  import { onDestroy } from 'svelte'
  import type { Message } from '../features/conversations/types.svelte'
  import { renderMarkdown, chatLinks } from '../lib/markdown'
  import { attachmentDataURL, formatAttachmentSize, isPreviewableImage } from '../features/conversations/attachments'

  type Props = {
    message: Message
    isStreaming?: boolean
  }

  let { message, isStreaming = false }: Props = $props()

  let rendered = $state('')
  let copied = $state(false)
  let renderTimer: ReturnType<typeof setTimeout> | null = null
  let copiedTimer: ReturnType<typeof setTimeout> | null = null
  let renderedContentEl = $state<HTMLDivElement | null>(null)
  let lastRenderedContent = ''

  $effect(() => {
    renderContentThrottled(message.content, isStreaming)
  })

  function renderContentThrottled(content: string, streaming: boolean) {
    if (!content) {
      rendered = ''
      lastRenderedContent = ''
      return
    }

    if (!streaming) {
      if (renderTimer) {
        clearTimeout(renderTimer)
        renderTimer = null
      }
      doRender(content)
      return
    }

    if (renderTimer) return
    renderTimer = setTimeout(() => {
      renderTimer = null
      doRender(message.content)
    }, 60)
  }

  function doRender(content: string) {
    if (content === lastRenderedContent) return
    lastRenderedContent = content
    rendered = renderMarkdown(content)
  }

  onDestroy(() => {
    if (renderTimer) clearTimeout(renderTimer)
    if (copiedTimer) clearTimeout(copiedTimer)
  })

  async function copyContent() {
    try {
      const plainText = message.role === 'assistant' && renderedContentEl
        ? renderedContentEl.innerText
        : message.content

      if (message.role === 'assistant' && renderedContentEl && navigator.clipboard.write && typeof ClipboardItem !== 'undefined') {
        const clone = renderedContentEl.cloneNode(true) as HTMLDivElement
        clone.querySelectorAll('[data-copy-ignore]').forEach((node) => node.remove())
        const html = clone.innerHTML
        try {
          await navigator.clipboard.write([
            new ClipboardItem({
              'text/plain': new Blob([plainText], { type: 'text/plain' }),
              'text/html': new Blob([html], { type: 'text/html' }),
            }),
          ])
        } catch {
          await navigator.clipboard.writeText(plainText)
        }
      } else {
        await navigator.clipboard.writeText(plainText)
      }

      copied = true
      if (copiedTimer) clearTimeout(copiedTimer)
      copiedTimer = setTimeout(() => {
        copied = false
      }, 2000)
    } catch (err) {
      console.error('Failed to copy message:', err)
    }
  }

  function formatTime(ts?: number): string {
    if (!ts) return ''
    return new Date(ts).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }
</script>

{#if message.role === 'user'}
  <div class="flex justify-end mb-4 group animate-fade-in pl-8 sm:pl-16">
    <div class="max-w-[85%] sm:max-w-[80%] flex flex-col items-end">
      {#if message.attachments.length}
        <div class="flex flex-wrap justify-end gap-2 mb-2" aria-label="Tệp đã gửi">
          {#each message.attachments as attachment (attachment.id)}
            {#if attachment.content && isPreviewableImage(attachment.mimeType)}
              <div class="sent-image" title={`${attachment.fileName} · ${formatAttachmentSize(attachment.size)}`}>
                <img src={attachmentDataURL(attachment)} alt={attachment.fileName} />
              </div>
            {:else}
              <div class="sent-file" title={`${attachment.fileName} · ${formatAttachmentSize(attachment.size)}`}>
                <svg class="w-4 h-4 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8zM14 2v6h6M8 13h8M8 17h6" /></svg>
                <span class="min-w-0"><span class="block max-w-48 truncate font-medium">{attachment.fileName}</span><span class="block text-2xs opacity-70">{formatAttachmentSize(attachment.size)}</span></span>
              </div>
            {/if}
          {/each}
        </div>
      {/if}
      {#if message.content.trim()}
        <div class="bg-mm-user-bubble text-white rounded-2xl rounded-tr-sm px-4 py-2.5 text-sm leading-relaxed shadow-xs whitespace-pre-wrap break-words">
          {message.content}
        </div>
      {/if}
      <div class="flex items-center justify-end gap-2 mt-1 opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 transition-opacity">
        {#if message.createdAt}
          <span class="text-xs text-mm-secondary">{formatTime(message.createdAt)}</span>
        {/if}
        <button
          type="button"
          class="text-xs text-mm-secondary hover:text-mm-text transition-colors cursor-pointer"
          onclick={copyContent}
          title="Sao chép nội dung"
        >
          {copied ? 'Đã sao chép' : 'Sao chép'}
        </button>
      </div>
    </div>
  </div>
<!-- An assistant row with no text is a tool-only agent step: render nothing
     instead of an avatar bubble with an empty body. -->
{:else if message.role === 'assistant' && (message.content.trim() || isStreaming)}
  <div class="flex items-start gap-3 mb-5 group animate-fade-in pr-6 sm:pr-14">
    <div class="w-6 h-6 flex-shrink-0 rounded-md bg-mm-panel border border-mm-border flex items-center justify-center p-0.5 mt-0.5 overflow-hidden shadow-xs">
      <img src="/tack.png" alt="Tack" class="w-full h-full object-contain" />
    </div>
    <div class="flex-1 min-w-0 max-w-[92%] sm:max-w-[88%]">
      {#if rendered}
        <div class="prose-notion" use:chatLinks bind:this={renderedContentEl}>
          {@html rendered}
          {#if isStreaming}
            <span data-copy-ignore class="inline-block w-1.5 h-4 bg-mm-accent ml-1 align-text-bottom rounded-xs animate-pulse" aria-hidden="true"></span>
          {/if}
        </div>
      {:else if isStreaming}
        <div class="flex items-center gap-1.5 py-2">
          <span class="thinking-dot"></span>
          <span class="thinking-dot"></span>
          <span class="thinking-dot"></span>
        </div>
      {/if}

      {#if message.content}
        <div class="flex items-center gap-2 mt-2 opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 transition-opacity">
          {#if message.createdAt}
            <span class="text-xs text-mm-secondary">{formatTime(message.createdAt)}</span>
          {/if}
          <button
            type="button"
            class="text-xs text-mm-secondary hover:text-mm-text transition-colors flex items-center gap-1 cursor-pointer"
            onclick={copyContent}
            title="Sao chép câu trả lời"
          >
            {#if copied}
              <svg class="w-3.5 h-3.5 text-mm-success" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
              </svg>
              <span class="text-mm-success">Đã sao chép</span>
            {:else}
              <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
              </svg>
              <span>Sao chép</span>
            {/if}
          </button>
        </div>
      {/if}
    </div>
  </div>
{/if}

<style>
  .sent-image {
    width: min(220px, 56vw);
    max-height: 220px;
    overflow: hidden;
    border: 1px solid var(--mm-border);
    border-radius: 12px;
    background: var(--mm-panel);
    box-shadow: var(--shadow-xs, 0 1px 2px rgb(0 0 0 / 0.08));
  }
  .sent-image img { display: block; width: 100%; max-height: 220px; object-fit: contain; }
  .sent-file {
    display: flex;
    max-width: min(280px, 70vw);
    align-items: center;
    gap: 8px;
    padding: 8px 10px;
    border: 1px solid var(--mm-border);
    border-radius: 10px;
    background: var(--mm-panel);
    color: var(--mm-text);
    font-size: 12px;
    text-align: left;
  }
</style>
