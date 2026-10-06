<script lang="ts">
  import { toast } from 'svelte-sonner'
  import { onMount } from 'svelte'
  import { localeTag, t } from '../../lib/i18n.svelte'
  import { desktop, type ZaloConfigUpdate, type ZaloStatusInfo } from '../../platform/desktop'

  let zaloEnabled = $state(false)
  let zaloToken = $state('')
  let zaloHasToken = $state(false)
  let zaloStatus = $state<ZaloStatusInfo | null>(null)
  let zaloSaving = $state(false)
  let zaloBusy = $state(false)
  let zaloPairingCode = $state('')
  let zaloPairingExpires = $state(0)
  let zaloPairedChats = $state<string[]>([])
  async function loadZalo() {

    try {
      const config = await desktop.getZaloConfig()
      zaloEnabled = config.enabled
      zaloHasToken = config.has_token
      zaloPairingCode = config.pairing_code
      zaloPairingExpires = config.pairing_expires_at ?? 0
      zaloPairedChats = config.paired_chats ?? []
      zaloStatus = await desktop.zaloStatus()
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : String(cause))
    }
  }
  async function saveZalo() {
    zaloSaving = true
    try {
      const update: ZaloConfigUpdate = { enabled: zaloEnabled }
      if (zaloToken.trim()) update.token = zaloToken.trim()
      zaloStatus = await desktop.saveZaloConfig(update)
      zaloHasToken = zaloStatus.configured
      zaloToken = ''
      zaloPairingCode = zaloStatus.pairing_code ?? ''
      zaloPairingExpires = zaloStatus.pairing_expires_at ?? 0
      zaloPairedChats = zaloStatus.paired_chat_ids ?? []
      toast.success(zaloStatus.bot_name ? t('zalo.toastLinked', { name: zaloStatus.bot_name }) : t('zalo.toastSaved'))
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : String(cause))
    } finally {
      zaloSaving = false
    }
  }

  async function runZalo(action: () => Promise<unknown>, ok: string) {
    zaloBusy = true
    try {
      zaloStatus = (await action()) as ZaloStatusInfo
      zaloHasToken = zaloStatus.configured
      zaloPairedChats = zaloStatus.paired_chat_ids ?? []
      zaloPairingCode = zaloStatus.pairing_code ?? ''
      zaloPairingExpires = zaloStatus.pairing_expires_at ?? 0
      toast.success(ok)
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : String(cause))
    } finally {
      zaloBusy = false
    }
  }

  async function unpairZalo(chatID: string) {
    await runZalo(() => desktop.unpairZaloChat(chatID), t('zalo.toastUnpaired', { chat: chatID }))
  }

  async function removeZalo() {
    await runZalo(() => desktop.removeZaloToken(), t('zalo.toastDisconnected'))
    if (!zaloHasToken) zaloEnabled = false
  }
  onMount(() => { void loadZalo() })
</script>

<section class="setting-section">
  <div class="section-title">Zalo</div>
  <label class="toggle-row">
    <span><strong>{t('zalo.connectTitle')}</strong><small>{t('zalo.connectDesc')}</small></span>
    <input type="checkbox" bind:checked={zaloEnabled} />
  </label>

  <label class="field-label" for="zalo-token">Bot token {zaloHasToken ? t('zalo.tokenSaved') : ''}</label>
  <input id="zalo-token" class="field font-mono" type="password" bind:value={zaloToken} autocomplete="off" placeholder={zaloHasToken ? '••••••••' : t('zalo.tokenPlaceholder')} />
  <p class="hint">{t('zalo.tokenHint')}</p>

  {#if zaloPairingCode}
    <div class="notice">
      <strong>{t('zalo.pairingCode')}</strong> <code class="text-base">{zaloPairingCode}</code>
      <p class="hint">{t('zalo.pairingHint', { code: zaloPairingCode })}</p>
      <button type="button" class="btn-notion text-xs" disabled={zaloBusy} onclick={() => runZalo(() => desktop.regenerateZaloPairingCode(), t('zalo.toastNewCode'))}>{t('zalo.newCode')}</button>
    </div>
  {/if}

  {#if zaloPairedChats.length}
    <div>
      <div class="field-label">{t('zalo.paired')}</div>
      <div class="flex flex-wrap gap-2">
        {#each zaloPairedChats as chatID (chatID)}
          <span class="flex items-center gap-1 px-2 py-1 rounded-md bg-mm-panel border border-mm-border text-xs">
            <code>{chatID}</code>
            <button type="button" class="text-red-500 px-1" disabled={zaloBusy} onclick={() => unpairZalo(chatID)} title={t('zalo.unpair')}>×</button>
          </span>
        {/each}
      </div>
    </div>
  {/if}

  {#if zaloStatus}
    <div class="notice">
      {#if zaloStatus.bot_name}
        <div><strong>Bot:</strong> {zaloStatus.bot_name}{zaloStatus.running ? ` · ${t('zalo.running')}` : ''}{zaloStatus.token_suffix ? ` · ••••${zaloStatus.token_suffix}` : ''}</div>
      {/if}
      {#if zaloStatus.last_error}<div class="text-red-500"><strong>{t('zalo.error')}</strong> {zaloStatus.last_error}</div>{/if}
      {#if !zaloStatus.bot_name && !zaloStatus.last_error}<div>{t('zalo.notConnected')}</div>{/if}
    </div>
  {/if}

  {#if zaloPairingCode && zaloPairingExpires}
  <p class="hint">{t('zalo.expiresHint', { time: new Date(zaloPairingExpires * 1000).toLocaleString(localeTag()) })}</p>
{/if}
<div class="flex flex-wrap justify-end gap-2 pt-2">
    <button type="button" class="btn-notion text-xs" disabled={zaloBusy || !zaloHasToken} onclick={() => runZalo(() => desktop.testZaloConnection(), t('zalo.toastTestOk'))}>{t('zalo.testConnection')}</button>
    <button type="button" class="btn-danger text-xs" disabled={zaloBusy || !zaloHasToken} onclick={removeZalo}>{t('zalo.disconnect')}</button>
    <button type="button" class="px-3 py-1.5 rounded-md bg-mm-accent text-white text-xs font-medium disabled:opacity-40" disabled={zaloSaving || (zaloEnabled && !zaloHasToken && !zaloToken.trim())} onclick={saveZalo}>
      {zaloSaving ? t('zalo.saving') : t('zalo.saveConnect')}
    </button>
  </div>
</section>
