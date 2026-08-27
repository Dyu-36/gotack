<script lang="ts">
  import {
    CRUSH_MODELS,
    CRUSH_PROVIDERS,
    REASONING_EFFORT_OPTIONS,
  } from '../features/conversations/conversation-state.svelte'
  import type { ModelType, ReasoningEffort } from '../features/conversations/types'

  type Props = {
    value: string
    modelLabel?: string
    thinkingLabel?: string
    isStreaming?: boolean
    selectedModelId?: string
    selectedThinkingId?: string
    onInput: (value: string) => void
    onSend: () => void
    onStop?: () => void
    onSelectModel?: (id: string, label: string, providerId?: string, type?: ModelType) => void
    onSelectThinking?: (id: ReasoningEffort) => void
    onOpenSettings?: () => void
  }

  let {
    value,
    modelLabel = 'Qwen 3.7 Plus',
    thinkingLabel = 'Think: High',
    isStreaming = false,
    selectedModelId = 'qwen3.7-plus',
    selectedThinkingId = 'high',
    onInput,
    onSend,
    onStop = () => {},
    onSelectModel = () => {},
    onSelectThinking = () => {},
    onOpenSettings = () => {},
  }: Props = $props()

  let textarea = $state<HTMLTextAreaElement>()
  let modelMenuOpen = $state(false)
  let thinkingMenuOpen = $state(false)
  let modelSearch = $state('')
  let activeModelType = $state<ModelType>('large')

  let canSend = $derived(value.trim().length > 0 && !isStreaming)

  let filteredModels = $derived(
    CRUSH_MODELS.filter(
      (m) =>
        m.name.toLowerCase().includes(modelSearch.toLowerCase()) ||
        m.id.toLowerCase().includes(modelSearch.toLowerCase()) ||
        (m.tag && m.tag.toLowerCase().includes(modelSearch.toLowerCase())),
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

  function pickModel(id: string, name: string, providerId: string) {
    onSelectModel(id, name, providerId, activeModelType)
    modelMenuOpen = false
    modelSearch = ''
  }

  function pickThinking(id: ReasoningEffort) {
    onSelectThinking(id)
    thinkingMenuOpen = false
  }
</script>

<div class="w-full max-w-input-w mx-auto px-4 pb-4 relative">
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
        <!-- Reasoning / Thinking Selector Menu -->
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
              <div class="px-2 py-1 text-2xs uppercase tracking-wider text-mm-tertiary font-semibold">Reasoning Effort (Crush)</div>
              {#each REASONING_EFFORT_OPTIONS as opt (opt.id)}
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

        <!-- Model Selector Menu (Crush Switch Model) -->
        <div class="relative">
          <button
            type="button"
            class="mm-model-select flex items-center gap-1.5 px-2 text-xs"
            onclick={() => { modelMenuOpen = !modelMenuOpen; thinkingMenuOpen = false }}
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
                
                <!-- Model Type Radio Toggle (Crush Native: Large Task vs Small Task) -->
                <div class="flex items-center gap-1 bg-mm-panel p-0.5 rounded-md border border-mm-border/50 text-2xs">
                  <button
                    type="button"
                    class="px-2 py-0.5 rounded font-medium transition-colors"
                    class:bg-mm-accent={activeModelType === 'large'}
                    class:text-white={activeModelType === 'large'}
                    class:text-mm-secondary={activeModelType !== 'large'}
                    onclick={(e) => { e.stopPropagation(); activeModelType = 'large' }}
                  >
                    Large Task
                  </button>
                  <button
                    type="button"
                    class="px-2 py-0.5 rounded font-medium transition-colors"
                    class:bg-mm-accent={activeModelType === 'small'}
                    class:text-white={activeModelType === 'small'}
                    class:text-mm-secondary={activeModelType !== 'small'}
                    onclick={(e) => { e.stopPropagation(); activeModelType = 'small' }}
                  >
                    Small Task
                  </button>
                </div>
              </div>

              <div class="my-1.5 px-0.5">
                <input
                  type="text"
                  bind:value={modelSearch}
                  placeholder={activeModelType === 'large' ? 'Choose model for large, complex tasks...' : 'Choose model for small, simple tasks...'}
                  aria-label="Tìm model"
                  class="w-full h-7 px-2 text-xs rounded bg-mm-panel border border-mm-border focus:border-mm-accent text-mm-text placeholder:text-mm-tertiary outline-none"
                  onclick={(e) => e.stopPropagation()}
                />
              </div>

              <div class="max-h-64 overflow-y-auto scroll-stable space-y-2 pr-0.5">
                {#each [...groupedModels.entries()] as [providerId, models]}
                  {@const prov = CRUSH_PROVIDERS.find((p) => p.id === providerId)}
                  <div class="space-y-0.5">
                    <div class="flex items-center justify-between px-1.5 py-0.5 text-2xs font-semibold text-mm-tertiary uppercase tracking-wider bg-mm-panel/40 rounded">
                      <span>{prov ? prov.name : providerId}</span>
                      {#if prov?.badge}
                        <span class="text-3xs px-1 py-0.2 rounded bg-mm-border/40 text-mm-secondary font-mono">{prov.badge}</span>
                      {/if}
                    </div>

                    {#each models as m (m.id)}
                      <button
                        type="button"
                        class="menu-item flex items-center justify-between group"
                        class:active={selectedModelId === m.id || modelLabel === m.name}
                        onclick={() => pickModel(m.id, m.name, m.providerId)}
                      >
                        <div class="flex-1 min-w-0 pr-2">
                          <div class="flex items-center gap-1.5">
                            <span class="font-medium truncate">{m.name}</span>
                            {#if m.canReason}
                              <span class="text-3xs px-1 py-0.2 rounded bg-mm-accent/15 text-mm-accent font-medium">Reasoning</span>
                            {/if}
                            {#if m.tag}
                              <span class="text-3xs px-1 py-0.2 rounded bg-mm-panel border border-mm-border/60 text-mm-tertiary font-mono">{m.tag}</span>
                            {/if}
                          </div>
                          {#if m.description}
                            <div class="text-2xs text-mm-tertiary truncate mt-0.5">{m.description}</div>
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
</style>

