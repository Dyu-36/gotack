<script lang="ts">
  import { toast } from 'svelte-sonner'
  import { onMount } from 'svelte'
  import { t } from '../../lib/i18n.svelte'
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
      toast.success(t('agent.toastToolsUpdated'))
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
      toast.success(trusted ? t('agent.toastTrusted') : t('agent.toastUntrusted'))
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
      toast.success(t('agent.toastResetTrust'))
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : String(cause))
    } finally {
      agentBusy = false
    }
  }
  onMount(() => { void loadAgentSettings() })
</script>

<section class="setting-section">
  <div class="section-title">{t('agent.toolsTitle')}</div>
  <p class="hint">{t('agent.toolsHint')}</p>
  {#each agentSettings.tools as tool (tool)}
    <label class="toggle-row">
      <span>
        <strong>{tool}</strong>
        <small>{tool === 'powershell' ? t('agent.powershellDesc') : t('agent.coreToolDesc')}</small>
      </span>
      <input
        type="checkbox"
        checked={!agentSettings.disabled_tools.includes(tool)}
        disabled={agentBusy}
        onchange={(event) => void toggleAgentTool(tool, event.currentTarget.checked)}
      />
    </label>
  {/each}

  <div class="section-title mt-3">{t('agent.trustTitle')}</div>
  {#if workspaceInfo}
    <div class="notice">
      <div><strong>{t('agent.workspace')}:</strong> <code>{workspaceInfo.path}</code></div>
      <div><strong>{t('agent.status')}:</strong> {workspaceInfo.trusted ? t('agent.statusTrusted') : workspaceInfo.trust_required ? t('agent.statusUndecided') : t('agent.statusUntrusted')}</div>
      {#if workspaceInfo.protected_resources?.length}
        <p class="hint">{t('agent.dynamicResources', { list: workspaceInfo.protected_resources.join(', ') })}</p>
      {:else}
        <p class="hint">{t('agent.noDynamic')}</p>
      {/if}
    </div>
    <div class="flex flex-wrap justify-end gap-2">
      <button type="button" class="btn-notion text-xs" disabled={agentBusy} onclick={() => void setProjectTrust(false)}>{t('agent.untrust')}</button>
      <button type="button" class="btn-notion text-xs" disabled={agentBusy} onclick={() => void resetProjectTrust()}>{t('agent.inherit')}</button>
      <button type="button" class="px-3 py-1.5 rounded-md bg-mm-accent text-white text-xs font-medium disabled:opacity-40" disabled={agentBusy} onclick={() => void setProjectTrust(true)}>{t('agent.trustProject')}</button>
    </div>
  {/if}
</section>
