<script lang="ts">
  // Keep the existing Settings import; this is now a read-only context panel.
  import { onMount } from 'svelte'
  import { desktop, type AssistantContextInfo } from '../platform/desktop'

  let info = $state<AssistantContextInfo | null>(null)
  let error = $state('')
  let loading = $state(false)

  async function refresh() {
    loading = true
    error = ''
    try { info = await desktop.assistantContextInfo() }
    catch (cause) { error = cause instanceof Error ? cause.message : String(cause) }
    finally { loading = false }
  }

  onMount(() => { void refresh() })
</script>

<section class="assistant-context">
  <header>
    <div><h3>Personal assistant context</h3><p>Small permanent context. Older conversations are searched only when needed.</p></div>
    <button type="button" onclick={() => void refresh()} disabled={loading}>{loading ? 'Loading...' : 'Refresh'}</button>
  </header>
  {#if error}<p role="alert">{error}</p>{/if}
  {#if info}
    <dl>
      <dt>Core</dt><dd>Built into the application, not loaded from external prompt files.</dd>
      <dt>PROFILE.md</dt><dd>{info.snapshot.profile_chars} / {info.profile_cap_chars} characters</dd>
      <dt>MEMORY.md</dt><dd>{info.snapshot.memory_chars} / {info.memory_cap_chars} characters</dd>
      <dt>Persistent payload</dt><dd>{info.snapshot.payload_bytes} / {info.snapshot.max_bytes || 24576} bytes</dd>
      <dt>Snapshot</dt><dd>{info.snapshot.layout_version ? `${info.snapshot.files} files; ${info.snapshot.reused ? 'reused' : 'built'} in ${info.snapshot.build_micros} microseconds` : 'Not loaded yet'}</dd>
      <dt>Personal data folder</dt><dd class="path">{info.directory}</dd>
    </dl>
    <p>These are character and byte limits, not model-specific token counts. Chat history, tools and attachments have separate engine budgets.</p>
    {#if info.snapshot.omitted_entries > 0}
      <p role="status">Some entries or invalid/oversized files were excluded from the snapshot. Originals were not changed. Consolidate personal memory rather than increasing the limits.</p>
    {/if}
    {#if info.import.needs_review?.length}
      <h4>Preserved older context</h4>
      <p>Old custom instructions were not automatically promoted into the new profile. Review the originals below and copy only preferences you still need.</p>
      {#each info.import.needs_review as path}<p class="path">{path}</p>{/each}
      {#if info.import.backup_dir}<p>Backup folder: <span class="path">{info.import.backup_dir}</span></p>{/if}
    {/if}
  {/if}
</section>

<style>
  .assistant-context { display: grid; gap: 14px; font-size: 13px; line-height: 1.6; }
  header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
  h3, h4, p { margin: 0; }
  h3 { font-size: 16px; font-weight: 600; }
  p { opacity: .8; }
  dl { display: grid; grid-template-columns: 150px minmax(0, 1fr); gap: 10px 16px; margin: 0; }
  dt { font-weight: 600; }
  dd { margin: 0; }
  .path { overflow-wrap: anywhere; font-family: ui-monospace, monospace; font-size: 12px; }
  button { border: 1px solid var(--mm-border); border-radius: 6px; padding: 5px 10px; cursor: pointer; }
  button:disabled { opacity: .5; cursor: default; }
</style>
