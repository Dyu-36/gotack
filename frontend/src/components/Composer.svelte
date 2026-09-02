<script lang="ts">
  import { catalog, REASONING_EFFORT_OPTIONS } from '../features/conversations/catalog.svelte'
  import { attachmentDataURL, formatAttachmentSize, isPreviewableImage } from '../features/conversations/attachments'
  import type { ChatAttachment, ReasoningEffort } from '../features/conversations/types.svelte'

  type Props = {
    value: string
    modelLabel?: string
    thinkingLabel?: string
    isStreaming?: boolean
    ready?: boolean
    selectedModelId?: string
    selectedProviderId?: string
    selectedThinkingId?: string
    attachments?: ChatAttachment[]
    conversationStarted?: boolean
    onInput: (value: string) => void
    onSend: () => void
    onAttachFiles?: (files: File[]) => void | Promise<void>
    // When present, the paperclip asks the host for a native dialog so a picked
    // file travels as a path instead of base64 through the webview.
    onPickFiles?: () => void | Promise<void>
    onRemoveAttachment?: (id: string) => void
    onStop?: () => void
    onSelectModel?: (id: string, label: string, providerId?: string) => void
    onSelectThinking?: (id: ReasoningEffort) => void
    onOpenSettings?: () => void
  }

  let {
    value,
    modelLabel = 'Model mặc định',
    thinkingLabel = 'Think: High',
    isStreaming = false,
    ready = false,
    selectedModelId = '',
    selectedProviderId = '',
    selectedThinkingId = 'high',
    attachments = [],
    conversationStarted = false,
    onInput,
    onSend,
    onAttachFiles = () => {},
    onPickFiles,
    onRemoveAttachment = () => {},
    onStop = () => {},
    onSelectModel = () => {},
    onSelectThinking = () => {},
    onOpenSettings = () => {},
  }: Props = $props()

  let textarea: HTMLTextAreaElement | undefined = $state()
  let fileInput: HTMLInputElement | undefined = $state()
  let modelMenuOpen = $state(false)
  let thinkingMenuOpen = $state(false)
  let modelSearch = $state('')

  let canSend = $derived((value.trim().length > 0 || attachments.length > 0) && !isStreaming)

  let selectedModel = $derived(
    catalog.configuredModels.find((m) => m.id === selectedModelId && (!selectedProviderId || m.providerId === selectedProviderId)),
  )

  let hasImageAttachments = $derived(attachments.some((a) => isPreviewableImage(a.mimeType)))

  let thinkingOptions = $derived.by(() => {
    if (!selectedModel) return REASONING_EFFORT_OPTIONS
    if (!selectedModel.can_reason) return REASONING_EFFORT_OPTIONS.filter((opt) => opt.id === 'none')

    if (selectedModel.reasoning_levels?.length) {
      const available: typeof REASONING_EFFORT_OPTIONS = []
      for (const level of selectedModel.reasoning_levels) {
        const option = REASONING_EFFORT_OPTIONS.find((opt) => opt.id === level)
        if (option) available.push(option)
      }
      if (available.length) return available
    }

    return [
      { id: 'none' as ReasoningEffort, label: 'Off (Tắt suy luận)', short: 'Off' },
      { id: 'high' as ReasoningEffort, label: 'On (Bật suy luận)', short: 'On' },
    ]
  })

  export function focus() {
    textarea?.focus()
  }

  let filteredModels = $derived(
    catalog.configuredModels.filter(
      (m) =>
        m.name.toLowerCase().includes(modelSearch.toLowerCase()) ||
        m.id.toLowerCase().includes(modelSearch.toLowerCase()) ||
        (catalog.provider(m.providerId)?.name.toLowerCase().includes(modelSearch.toLowerCase()) ?? false),
    ),
  )

  let groupedModels = $derived.by(() => {
    const map = new Map<string, typeof filteredModels>()
    for (const model of filteredModels) {
      const list = map.get(model.providerId) ?? []
      list.push(model)
      map.set(model.providerId, list)
    }
    return map
  })

  function modelMeta(model: (typeof filteredModels)[number]): string {
    const parts: string[] = []
    if (model.context_window && model.context_window >= 1000) parts.push(`${Math.round(model.context_window / 1000)}K context`)
    if (model.cost_per_1m_in || model.cost_per_1m_out) {
      parts.push(`$${model.cost_per_1m_in ?? 0}/$${model.cost_per_1m_out ?? 0} per 1M`)
    }
    return parts.join(' · ')
  }

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

  function handleFileSelection(event: Event) {
    const input = event.currentTarget as HTMLInputElement
    const files = Array.from(input.files ?? [])
    input.value = ''
    if (files.length) void onAttachFiles(files)
  }

  function handlePaste(event: ClipboardEvent) {
    const files = Array.from(event.clipboardData?.items ?? [])
      .filter((item) => item.kind === 'file' && item.type.startsWith('image/'))
      .map((item) => item.getAsFile())
      .filter((file): file is File => file !== null)
    if (!files.length) return
    event.preventDefault()
    void onAttachFiles(files)
  }

  function pickModel(id: string, name: string, providerId: string) {
    onSelectModel(id, name, providerId)
    modelMenuOpen = false
    modelSearch = ''
  }

  function pickThinking(id: ReasoningEffort) {
    onSelectThinking(id)
    thinkingMenuOpen = false
  }
</script>

<div class="w-full mx-auto px-4 pb-4 relative {conversationStarted ? 'max-w-5xl' : 'max-w-input-w'}">
  <input
    bind:this={fileInput}
    type="file"
    multiple
    class="hidden"
    onchange={handleFileSelection}
    aria-label="Tải lên tệp đính kèm"
  />

  <div class="rounded-mm border border-mm-border bg-mm-bg transition-all focus-within:border-mm-accent/60 focus-within:ring-2 focus-within:ring-mm-accent/15 shadow-sm">
    {#if attachments.length}
      <div class="flex flex-wrap gap-2 px-3 pt-3" aria-label="Tệp đính kèm">
        {#each attachments as attachment (attachment.id)}
          <div class="attachment-preview group/attachment" title={`${attachment.fileName} · ${formatAttachmentSize(attachment.size)}`}>
            {#if attachment.content && isPreviewableImage(attachment.mimeType)}
              <img src={attachmentDataURL(attachment)} alt={attachment.fileName} />
            {:else}
              <svg class="w-5 h-5 text-mm-secondary shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8zM14 2v6h6M8 13h8M8 17h6" /></svg>
              <span class="max-w-32 truncate text-xs text-mm-text">{attachment.fileName}</span>
            {/if}
            <button
              type="button"
              class="attachment-remove"
              aria-label={`Gỡ tệp ${attachment.fileName}`}
              title="Gỡ tệp"
              onclick={() => onRemoveAttachment(attachment.id)}
            >×</button>
          </div>
        {/each}
      </div>
      {#if hasImageAttachments && selectedModel && !selectedModel.supports_vision}
        <div class="px-3 pt-1.5 pb-0 text-3xs text-mm-tertiary flex items-center gap-1">
          <span class="text-amber-500 font-semibold">ℹ️ Text-only Model:</span>
          <span>Gotack sẽ tự động OCR bóc tách nội dung văn bản từ ảnh khi gửi.</span>
        </div>
      {/if}
    {/if}

    <div class="px-4 pt-3.5 pb-2">
      <textarea
        bind:this={textarea}
        value={value}
        oninput={(event) => { onInput(event.currentTarget.value); autoResize() }}
        onkeydown={handleKeydown}
        onpaste={handlePaste}
        placeholder="Nhập tin nhắn hoặc dán ảnh... (Enter để gửi, Shift+Enter để xuống dòng)"
        rows="1"
        disabled={isStreaming}
        aria-busy={!ready}
        aria-label="Nội dung tin nhắn"
        class="w-full resize-none bg-transparent text-mm-text text-base leading-relaxed placeholder:text-mm-tertiary overflow-y-auto scroll-stable min-h-6 max-h-[var(--composer-max-h)] disabled:opacity-60 focus:outline-none focus-visible:outline-none focus:ring-0"
      ></textarea>
    </div>

    <div class="flex items-center justify-between px-3 pb-3">
      <div class="flex items-center gap-1">
        <button
          type="button"
          class="p-1.5 rounded text-mm-tertiary hover:text-mm-text hover:bg-mm-hover transition-colors"
          title="Đính kèm tệp hoặc ảnh"
          aria-label="Đính kèm tệp"
          onclick={() => (onPickFiles ? void onPickFiles() : fileInput?.click())}
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13" /></svg>
        </button>
      </div>

      <div class="flex items-center gap-1">
        <div class="relative">
          <button
            type="button"
            class="mm-model-select flex items-center gap-1.5 px-2 text-xs"
            onclick={() => { thinkingMenuOpen = !thinkingMenuOpen; modelMenuOpen = false }}
            aria-label="Chọn mức độ suy luận (Reasoning Effort)"
          >
            <svg class="w-3.5 h-3.5 text-mm-secondary" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" /></svg>
            <span>{thinkingLabel}</span>
            <svg class="w-3 h-3 text-mm-tertiary" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" /></svg>
          </button>

          {#if thinkingMenuOpen}
            <div class="fixed inset-0 z-20" onclick={() => (thinkingMenuOpen = false)} aria-hidden="true"></div>
            <div class="menu-pop absolute bottom-full right-0 mb-2 w-56 p-1.5 z-30 animate-fade-in">
              <div class="px-2 py-1 text-2xs uppercase tracking-wider text-mm-tertiary font-semibold">Reasoning Effort</div>
              {#each thinkingOptions as opt (opt.id)}
                <button
                  type="button"
                  class="menu-item flex items-center justify-between"
                  class:active={selectedThinkingId === opt.id}
                  onclick={() => pickThinking(opt.id)}
                >
                  <div class="flex items-center gap-1.5">
                    <span>{opt.label}</span>
                  </div>
                  {#if selectedThinkingId === opt.id}
                    <svg class="w-3.5 h-3.5 text-mm-accent" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" /></svg>
                  {/if}
                </button>
              {/each}
            </div>
          {/if}
        </div>

        <div class="relative">
          <button
            type="button"
            class="mm-model-select flex items-center gap-1.5 px-2 text-xs"
            onclick={() => { modelMenuOpen = !modelMenuOpen; thinkingMenuOpen = false; if (catalog.status === 'idle') catalog.refresh() }}
            aria-label="Chọn Model AI"
          >
            <span class="w-2 h-2 rounded-full bg-mm-accent"></span>
            <span class="max-w-36 truncate font-medium">{modelLabel}</span>
            <svg class="w-3 h-3 text-mm-tertiary" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" /></svg>
          </button>

          {#if modelMenuOpen}
            <div class="fixed inset-0 z-20" onclick={() => (modelMenuOpen = false)} aria-hidden="true"></div>
            <div class="menu-pop absolute bottom-full right-0 mb-2 w-80 p-2 z-30 animate-fade-in">
              <div class="flex items-center justify-between px-1 pb-1.5 border-b border-mm-border">
                <span class="text-2xs uppercase tracking-wider text-mm-tertiary font-bold">Switch Model</span>
              </div>

              <div class="my-1.5 px-0.5">
                <input
                  type="text"
                  bind:value={modelSearch}
                  placeholder="Chọn model cho agent..."
                  aria-label="Tìm model"
                  class="w-full h-7 px-2 text-xs rounded bg-mm-panel border border-mm-border focus:border-mm-accent text-mm-text placeholder:text-mm-tertiary outline-none"
                  onclick={(e) => e.stopPropagation()}
                />
              </div>

              <div class="max-h-64 overflow-y-auto scroll-stable space-y-2 pr-0.5">
                {#if catalog.status === 'loading'}
                  <div class="px-2 py-4 text-center text-xs text-mm-tertiary">Đang tải danh sách model...</div>
                {:else if catalog.status === 'error'}
                  <div class="px-2 py-4 text-center text-xs text-mm-tertiary">{catalog.error}</div>
                {:else}
                  {#each [...groupedModels.entries()] as [providerId, models] (providerId)}
                    <div class="space-y-0.5">
                      <div class="flex items-center justify-between px-1.5 py-0.5 text-2xs font-semibold text-mm-tertiary uppercase tracking-wider bg-mm-panel/40 rounded">
                        <span>{catalog.provider(providerId)?.name ?? providerId}</span>
                      </div>

                      {#each models as m (m.id)}
                        {@const meta = modelMeta(m)}
                        <button
                          type="button"
                          class="menu-item flex items-center justify-between group"
                          class:active={selectedModelId === m.id || modelLabel === m.name}
                          onclick={() => pickModel(m.id, m.name, m.providerId)}
                        >
                          <div class="flex-1 min-w-0 pr-2">
                            <div class="flex items-center gap-1.5">
                              <span class="font-medium truncate">{m.name}</span>
                              {#if m.supports_vision}
                                <span class="text-3xs px-1 py-0.2 rounded bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 font-medium">Vision</span>
                              {/if}
                              {#if m.can_reason}
                                <span class="text-3xs px-1 py-0.2 rounded bg-mm-accent/15 text-mm-accent font-medium">Reasoning</span>
                              {/if}
                            </div>
                            {#if meta}
                              <div class="text-2xs text-mm-tertiary truncate mt-0.5">{meta}</div>
                            {/if}
                          </div>

                          {#if selectedModelId === m.id || modelLabel === m.name}
                            <svg class="w-3.5 h-3.5 text-mm-accent shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" /></svg>
                          {/if}
                        </button>
                      {/each}
                    </div>
                  {/each}

                  {#if filteredModels.length === 0}
                    <div class="px-2 py-4 text-center text-xs text-mm-tertiary">Không tìm thấy model phù hợp</div>
                  {/if}
                {/if}
              </div>

              <div class="mt-2 pt-1.5 border-t border-mm-border">
                <button
                  type="button"
                  class="w-full flex items-center justify-center gap-1.5 px-2 py-1 rounded text-2xs text-mm-secondary hover:text-mm-text hover:bg-mm-hover transition-colors"
                  onclick={() => { modelMenuOpen = false; onOpenSettings() }}
                >
                  <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" /><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" /></svg>
                  <span>Cấu hình Providers & Xác thực OAuth / API Key...</span>
                </button>
              </div>
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
    padding: 6px 8px;
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

  .attachment-preview {
    position: relative;
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 3px 6px;
    border: 1px solid var(--mm-border);
    border-radius: 8px;
    background: var(--mm-panel);
    max-width: 200px;
    height: 38px;
  }
  .attachment-preview img {
    width: 30px;
    height: 30px;
    object-fit: cover;
    border-radius: 4px;
  }
  .attachment-remove {
    position: absolute;
    top: -4px;
    right: -4px;
    width: 16px;
    height: 16px;
    border-radius: 999px;
    background: var(--mm-text);
    color: var(--mm-bg);
    font-size: 11px;
    font-weight: bold;
    line-height: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    border: 1px solid var(--mm-border);
    cursor: pointer;
    opacity: 0.85;
    transition: opacity 0.15s;
  }
  .attachment-remove:hover {
    opacity: 1;
  }
</style>
