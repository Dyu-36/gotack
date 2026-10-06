<script lang="ts">
  import { toast } from 'svelte-sonner'
  import { onMount } from 'svelte'
  import { desktop, type AgentSettingsInfo, type WorkspaceInfo } from '../../platform/desktop'

  let agentSettings = $state<AgentSettingsInfo>({ tools: [], disabled_tools: [] })
  let agentBusy = $state(false)
  let workspaceInfo = $state<WorkspaceInfo | null>(null)
  async function loadAgentSettings() {
    try {
      agentSettings = await desktop.getAgentSettings()
      workspaceInfo = await desktop.currentWorkspace()
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : String(cause))
    }
  }

  async function toggleAgentTool(tool: string, enabled: boolean) {
    const disabled = new Set(agentSettings.disabled_tools)
    if (enabled) disabled.delete(tool)
    else disabled.add(tool)
    agentBusy = true
    try {
      agentSettings = await desktop.saveAgentSettings([...disabled])
      toast.success('Đã cập nhật tool của agent')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : String(cause))
    } finally {
      agentBusy = false
    }
  }

  async function setProjectTrust(trusted: boolean) {
    agentBusy = true
    try {
      workspaceInfo = await desktop.setWorkspaceTrust(trusted)
      toast.success(trusted ? 'Đã trust project' : 'Đã chặn tài nguyên động của project')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : String(cause))
    } finally {
      agentBusy = false
    }
  }

  async function resetProjectTrust() {
    agentBusy = true
    try {
      workspaceInfo = await desktop.resetWorkspaceTrust()
      toast.success('Đã xóa quyết định trust riêng của project')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : String(cause))
    } finally {
      agentBusy = false
    }
  }
  onMount(() => { void loadAgentSettings() })
</script>

<section class="setting-section">
  <div class="section-title">Công cụ agent</div>
  <p class="hint">Mặc định Gotack bật toàn bộ 6 công cụ lõi. Đây là cấu hình capability, không phải cơ chế xin quyền từng lần.</p>
  {#each agentSettings.tools as tool (tool)}
    <label class="toggle-row">
      <span>
        <strong>{tool}</strong>
        <small>{tool === 'powershell' ? 'Thực thi lệnh hệ thống với quyền của người dùng đang chạy Gotack.' : 'Công cụ lõi của agent.'}</small>
      </span>
      <input
        type="checkbox"
        checked={!agentSettings.disabled_tools.includes(tool)}
        disabled={agentBusy}
        onchange={(event) => void toggleAgentTool(tool, event.currentTarget.checked)}
      />
    </label>
  {/each}

  <div class="section-title mt-3">Project Trust</div>
  {#if workspaceInfo}
    <div class="notice">
      <div><strong>Workspace:</strong> <code>{workspaceInfo.path}</code></div>
      <div><strong>Trạng thái:</strong> {workspaceInfo.trusted ? 'Trusted' : workspaceInfo.trust_required ? 'Chưa quyết định' : 'Không trusted'}</div>
      {#if workspaceInfo.protected_resources?.length}
        <p class="hint">Tài nguyên động: {workspaceInfo.protected_resources.join(', ')}</p>
      {:else}
        <p class="hint">Workspace không có tài nguyên động cần trust.</p>
      {/if}
    </div>
    <div class="flex flex-wrap justify-end gap-2">
      <button type="button" class="btn-notion text-xs" disabled={agentBusy} onclick={() => void setProjectTrust(false)}>Không trust</button>
      <button type="button" class="btn-notion text-xs" disabled={agentBusy} onclick={() => void resetProjectTrust()}>Kế thừa</button>
      <button type="button" class="px-3 py-1.5 rounded-md bg-mm-accent text-white text-xs font-medium disabled:opacity-40" disabled={agentBusy} onclick={() => void setProjectTrust(true)}>Trust project</button>
    </div>
  {/if}
</section>
