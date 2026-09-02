<script lang="ts">
  import { slide } from 'svelte/transition'
  import type { Message } from '../features/conversations/types.svelte'
  import { formatToolGroupSummary } from '../lib/tool-display'
  import ToolCard from './ToolCard.svelte'

  type Props = {
    tools: Message[]
  }

  let { tools }: Props = $props()

  let expanded = $state(false)
  let summary = $derived(formatToolGroupSummary(tools))
</script>

<div
  class="thinking-card animate-tool-in mb-3 mr-8 sm:mr-20"
  class:is-expanded={expanded}
  role="region"
  aria-label={`Suy nghĩ (${summary})`}
>
  <button
    type="button"
    class="thinking-header w-full flex items-center justify-between gap-2.5 px-3 py-2 text-left cursor-pointer select-none bg-mm-panel hover:bg-mm-hover transition-colors rounded-lg"
    class:rounded-b-none={expanded}
    onclick={() => expanded = !expanded}
    aria-expanded={expanded}
  >
    <div class="flex items-center gap-2 min-w-0 flex-1">
      <div class="shrink-0 flex items-center justify-center w-4 h-4 text-mm-secondary">
        <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <path d="M9.5 2A2.5 2.5 0 0 1 12 4.5v15a2.5 2.5 0 0 1-4.96.44 2.5 2.5 0 0 1-2.96-3.08 3 3 0 0 1-.34-5.58 2.5 2.5 0 0 1 1.32-4.24 2.5 2.5 0 0 1 4.44-2.04Z"></path>
          <path d="M14.5 2A2.5 2.5 0 0 0 12 4.5v15a2.5 2.5 0 0 0 4.96.44 2.5 2.5 0 0 0 2.96-3.08 3 3 0 0 0 .34-5.58 2.5 2.5 0 0 0-1.32-4.24 2.5 2.5 0 0 0-4.44-2.04Z"></path>
        </svg>
      </div>

      <div class="flex items-baseline gap-1.5 min-w-0 truncate text-xs">
        <span class="font-medium text-mm-text shrink-0">Suy nghĩ</span>
        <span class="text-mm-tertiary truncate font-normal">· {summary}</span>
      </div>
    </div>

    <div class="flex items-center gap-2 shrink-0">
      <span class="status-pill text-2xs px-1.5 py-0.5 rounded font-medium bg-emerald-500-15 text-emerald-600 dark:text-emerald-400">
        hoàn thành
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

  {#if expanded}
    <div
      class="thinking-body border-t border-mm-border bg-mm-panel/40 p-2.5 flex flex-col gap-2"
      transition:slide={{ duration: 180 }}
    >
      {#each tools as tool (tool.id)}
        <ToolCard message={tool} nested={true} />
      {/each}
    </div>
  {/if}
</div>

<style>
  .thinking-card {
    margin-left: 36px;
    border: 1px solid var(--mm-border);
    border-radius: 9px;
    background: var(--mm-panel);
    transition: border-color 150ms ease, box-shadow 150ms ease;
  }

  .thinking-card:hover {
    border-color: var(--mm-border-strong);
  }

  .bg-emerald-500-15 {
    background-color: rgba(16, 185, 129, 0.12);
  }

  @media (prefers-reduced-motion: reduce) {
    .thinking-card,
    .chevron-icon {
      transition: none !important;
    }
  }
</style>
