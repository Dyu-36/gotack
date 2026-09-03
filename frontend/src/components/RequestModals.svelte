<script lang="ts">
  import type { PermissionRequestPayload } from '../platform/desktop'

  type Props = {
    permission: PermissionRequestPayload | null
    onPermission: (decision: 'allow' | 'allow_session' | 'deny') => void
    secondsLeft?: number
    expired?: boolean
  }

  let { permission, onPermission, secondsLeft = 0, expired = false }: Props = $props()
</script>

{#if permission}
  <div class="fixed inset-0 z-50 bg-black/35 backdrop-blur-sm flex items-center justify-center p-4">
    <div class="w-full max-w-lg rounded-xl border border-mm-border bg-mm-bg shadow-xl" role="dialog" aria-modal="true" aria-label="Permission request">
      <header class="px-5 py-4 border-b border-mm-border">
        <div class="text-xs font-semibold uppercase tracking-wider text-amber-500">Yêu cầu quyền</div>
        <h2 class="mt-1 text-base font-semibold text-mm-text">{permission.request.tool_name || 'Tool request'}{#if !expired && secondsLeft > 0 && secondsLeft <= 30} <span class="ml-2 text-xs text-mm-tertiary">{secondsLeft}s</span>{/if}</h2>
      </header>
      <div class="p-5 space-y-3">
        {#if permission.request.description}<p class="text-sm text-mm-text leading-relaxed">{permission.request.description}</p>{/if}
        {#if permission.request.path}<div class="rounded-md bg-mm-panel px-3 py-2 text-xs font-mono text-mm-secondary break-all">{permission.request.path}</div>{/if}
        {#if permission.request.params}
          <details class="text-xs text-mm-secondary">
            <summary class="cursor-pointer select-none">Tool parameters</summary>
            <pre class="mt-2 max-h-40 overflow-auto rounded-md bg-mm-panel p-2 text-2xs">{JSON.stringify(permission.request.params, null, 2)}</pre>
          </details>
        {/if}
      </div>
      <footer class="px-5 py-3 border-t border-mm-border flex justify-end gap-2">
        {#if expired}
          <span class="px-3 py-1.5 text-xs text-mm-tertiary" aria-live="polite">Hết hạn</span>
          <button type="button" class="px-3 py-1.5 rounded-md bg-mm-accent text-white text-xs font-medium" onclick={() => onPermission('deny')}>Đóng</button>
        {:else}
          <button type="button" class="btn-notion px-3 py-1.5 text-xs text-red-500" onclick={() => onPermission('deny')}>Từ chối</button>
          <button type="button" class="btn-notion px-3 py-1.5 text-xs" onclick={() => onPermission('allow_session')}>Cho phép trong session</button>
          <button type="button" class="px-3 py-1.5 rounded-md bg-mm-accent text-white text-xs font-medium" onclick={() => onPermission('allow')}>Cho phép một lần</button>
        {/if}
      </footer>
    </div>
  </div>
{/if}
