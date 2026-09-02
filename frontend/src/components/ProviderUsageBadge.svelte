<script lang="ts">
  import { onMount } from 'svelte'
  import {
    desktop,
    events,
    on,
    type ProviderUsageInfo,
    type SessionDoneEvent,
  } from '../platform/desktop'
  import { catalog } from '../features/conversations/catalog.svelte'
  import {
    formatUsageReset,
    orderedUsageWindows,
    remainingPercent,
    usageWindowLabel,
  } from '../features/conversations/provider-usage'

  type Props = {
    providerId: string
    ready?: boolean
  }

  let { providerId, ready = false }: Props = $props()

  let usage = $state<ProviderUsageInfo | null>(null)
  let loading = $state(false)
  let error = $state('')
  let open = $state(false)
  let generation = 0

  const windows = $derived(orderedUsageWindows(usage?.windows ?? []))
  const providerName = $derived(
    usage?.provider_name || catalog.provider(providerId)?.name || providerId,
  )
  const badgeText = $derived.by(() => {
    if (loading && !usage) return 'Đang tải hạn mức…'
    if (usage?.available && windows.length) {
      const visible = windows
        .slice(0, 2)
        .map((item) => `${usageWindowLabel(item)} ${remainingPercent(item)}%`)
      if (windows.length > 2) visible.push(`+${windows.length - 2}`)
      return visible.join(' · ')
    }
    return 'Hạn mức —'
  })

  function errorText(cause: unknown): string {
    return cause instanceof Error ? cause.message : String(cause)
  }

  async function refresh(id = providerId, showSpinner = true) {
    if (!id || !ready || !desktop.available()) return
    const request = ++generation
    if (showSpinner) loading = true
    try {
      const result = await desktop.getProviderUsage(id)
      if (request !== generation || id !== providerId) return
      usage = result
      error = ''
    } catch (cause) {
      if (request !== generation || id !== providerId) return
      error = errorText(cause)
    } finally {
      if (request === generation) loading = false
    }
  }

  function toggleOpen() {
    open = !open
    if (open) void refresh(providerId)
  }

  $effect(() => {
    const id = providerId
    const canLoad = ready && desktop.available() && id.length > 0
    if (!canLoad) {
      generation += 1
      usage = null
      loading = false
      error = ''
      open = false
      return
    }
    void refresh(id)
  })

  onMount(() => {
    if (!desktop.available()) return
    return on<SessionDoneEvent>(events.sessionDone, () => {
      if (ready && providerId) void refresh(providerId, false)
    })
  })
</script>

{#if providerId && ready}
  <div class="usage-root">
    <button
      type="button"
      class="usage-trigger"
      class:reached={usage?.limit_reached}
      aria-label={`Hạn mức ${providerName}`}
      aria-expanded={open}
      title={`Hạn mức ${providerName}`}
      onclick={toggleOpen}
    >
      <svg viewBox="0 0 24 24" aria-hidden="true">
        <path d="M4 19a8 8 0 1 1 16 0" />
        <path d="m12 11 3.5-3.5" />
        <path d="M8 19h8" />
      </svg>
      <span>{badgeText}</span>
      {#if loading}
        <span class="spinner" aria-label="Đang làm mới"></span>
      {/if}
    </button>

    {#if open}
      <button
        type="button"
        class="usage-dismiss"
        aria-label="Đóng hạn mức provider"
        onclick={() => (open = false)}
      ></button>
      <section class="usage-popover" aria-label={`Hạn mức ${providerName}`}>
        <header>
          <div class="provider-heading">
            <strong>{providerName || 'Provider'}</strong>
            {#if usage?.plan}
              <span>{usage.plan}</span>
            {/if}
          </div>
          <button
            type="button"
            class="refresh-button"
            title="Làm mới hạn mức"
            aria-label="Làm mới hạn mức"
            disabled={loading}
            onclick={() => void refresh(providerId)}
          >
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M20 11a8.1 8.1 0 0 0-15.5-2M4 4v5h5" />
              <path d="M4 13a8.1 8.1 0 0 0 15.5 2M20 20v-5h-5" />
            </svg>
          </button>
        </header>

        {#if error}
          <div class="usage-message" role="status">{error}</div>
        {:else if loading && !usage}
          <div class="usage-message" role="status">Đang đọc hạn mức từ provider…</div>
        {:else if usage?.available && windows.length}
          <div class="window-list">
            {#each windows as item (item.id)}
              {@const remaining = remainingPercent(item)}
              <div class="window-row">
                <div class="window-title">
                  <span>{usageWindowLabel(item)}</span>
                  <strong>{remaining}% còn lại</strong>
                </div>
                <div
                  class="usage-meter"
                  role="progressbar"
                  aria-label={`${usageWindowLabel(item)} còn lại`}
                  aria-valuemin="0"
                  aria-valuemax="100"
                  aria-valuenow={remaining}
                >
                  <span style={`width: ${remaining}%`}></span>
                </div>
                <div class="reset-at">Đặt lại {formatUsageReset(item.resets_at)}</div>
              </div>
            {/each}
          </div>
        {:else}
          <div class="usage-message">
            {usage?.unavailable_reason ?? 'Provider chưa cung cấp hạn mức qua API.'}
          </div>
        {/if}

        <footer>
          Provider trả về tỷ lệ quota; Gotack không suy đoán số token tuyệt đối.
        </footer>
      </section>
    {/if}
  </div>
{/if}

<style>
  .usage-root {
    position: relative;
    max-width: min(520px, calc(100vw - 32px));
  }

  .usage-trigger {
    display: inline-flex;
    max-width: 100%;
    min-height: 30px;
    align-items: center;
    gap: 6px;
    padding: 5px 9px;
    border: 1px solid var(--mm-border);
    border-radius: 999px;
    background: color-mix(in srgb, var(--mm-bg) 92%, transparent);
    box-shadow: 0 5px 18px rgb(0 0 0 / 12%);
    color: var(--mm-secondary);
    font: inherit;
    font-size: 11px;
    font-weight: 600;
    line-height: 1;
    cursor: pointer;
    backdrop-filter: blur(8px);
  }

  .usage-trigger:hover,
  .usage-trigger[aria-expanded='true'] {
    border-color: var(--mm-border-strong);
    background: var(--mm-hover);
    color: var(--mm-text);
  }

  .usage-trigger.reached {
    border-color: color-mix(in srgb, #f59e0b 58%, var(--mm-border));
  }

  .usage-trigger > span:not(.spinner) {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .usage-trigger svg,
  .refresh-button svg {
    width: 14px;
    height: 14px;
    flex: 0 0 auto;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.8;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .spinner {
    width: 10px;
    height: 10px;
    flex: 0 0 auto;
    border: 1.5px solid currentColor;
    border-right-color: transparent;
    border-radius: 999px;
    animation: usage-spin 650ms linear infinite;
  }

  .usage-dismiss {
    position: fixed;
    inset: 0;
    z-index: 20;
    border: 0;
    background: transparent;
    cursor: default;
  }

  .usage-popover {
    position: absolute;
    right: 0;
    bottom: calc(100% + 8px);
    z-index: 21;
    width: min(360px, calc(100vw - 32px));
    overflow: hidden;
    border: 1px solid var(--mm-border);
    border-radius: 12px;
    background: var(--mm-bg);
    box-shadow: var(--shadow-popup, 0 12px 36px rgb(0 0 0 / 22%));
    color: var(--mm-text);
  }

  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 12px 12px 10px;
    border-bottom: 1px solid var(--mm-border);
  }

  .provider-heading {
    display: flex;
    min-width: 0;
    align-items: baseline;
    gap: 7px;
  }

  .provider-heading strong {
    overflow: hidden;
    font-size: 12px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .provider-heading span {
    flex: 0 0 auto;
    padding: 2px 5px;
    border-radius: 999px;
    background: var(--mm-panel);
    color: var(--mm-tertiary);
    font-size: 9px;
    font-weight: 700;
    text-transform: uppercase;
  }

  .refresh-button {
    display: inline-flex;
    width: 26px;
    height: 26px;
    flex: 0 0 auto;
    align-items: center;
    justify-content: center;
    border: 0;
    border-radius: 6px;
    background: transparent;
    color: var(--mm-secondary);
    cursor: pointer;
  }

  .refresh-button:hover:not(:disabled) {
    background: var(--mm-hover);
    color: var(--mm-text);
  }

  .refresh-button:disabled {
    cursor: default;
    opacity: 0.45;
  }

  .window-list {
    display: grid;
    gap: 12px;
    padding: 12px;
  }

  .window-row {
    display: grid;
    gap: 6px;
  }

  .window-title {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    font-size: 11px;
  }

  .window-title > span {
    overflow: hidden;
    color: var(--mm-secondary);
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .window-title strong {
    flex: 0 0 auto;
    font-size: 11px;
  }

  .usage-meter {
    height: 5px;
    overflow: hidden;
    border-radius: 999px;
    background: var(--mm-panel);
  }

  .usage-meter > span {
    display: block;
    height: 100%;
    border-radius: inherit;
    background: var(--mm-accent);
    transition: width 180ms ease;
  }

  .reset-at {
    color: var(--mm-tertiary);
    font-size: 10px;
  }

  .usage-message {
    padding: 16px 12px;
    color: var(--mm-secondary);
    font-size: 11px;
    line-height: 1.5;
  }

  footer {
    padding: 8px 12px;
    border-top: 1px solid var(--mm-border);
    color: var(--mm-tertiary);
    font-size: 9px;
    line-height: 1.45;
  }

  @keyframes usage-spin {
    to { transform: rotate(360deg); }
  }

  @media (prefers-reduced-motion: reduce) {
    .spinner { animation-duration: 0ms; }
    .usage-meter > span { transition-duration: 0ms; }
  }
</style>
