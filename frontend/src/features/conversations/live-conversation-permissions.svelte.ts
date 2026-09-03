import { desktop, type PermissionRequestPayload as Envelope } from '../../platform/desktop'

export type PermissionDeps = {
  permission: { value: Envelope | null }
  reportError: (cause: unknown, prefix?: string) => void
}

export function createPermissionState(deps: PermissionDeps) {
  let now = $state(Date.now())
  let permissionSecondsLeft = $derived(
    deps.permission.value ? Math.max(0, Math.ceil((deps.permission.value.expires_at_ms - now) / 1000)) : 0,
  )
  let permissionExpired = $derived(
    deps.permission.value ? deps.permission.value.expires_at_ms > 0 && now >= deps.permission.value.expires_at_ms : false,
  )

  let permissionExpiryTimer: number | undefined
  $effect(() => {
    if (!deps.permission.value) return
    const tick = setInterval(() => { now = Date.now() }, 1000)
    return () => clearInterval(tick)
  })
  $effect(() => {
    if (!deps.permission.value || !permissionExpired) return
    if (permissionExpiryTimer !== undefined) window.clearTimeout(permissionExpiryTimer)
    permissionExpiryTimer = window.setTimeout(() => {
      permissionExpiryTimer = undefined
      deps.permission.value = null
    }, 2000)
    return () => {
      if (permissionExpiryTimer !== undefined) {
        window.clearTimeout(permissionExpiryTimer)
        permissionExpiryTimer = undefined
      }
    }
  })

  const answerPermission = async (decision: 'allow' | 'allow_session' | 'deny') => {
    const current = deps.permission.value
    if (!current) return
    const id = current.request.id
    deps.permission.value = null
    try { await desktop.answerPermission(id, decision) } catch (cause) { deps.reportError(cause, 'Permission response') }
  }

  return {
    now: { get value() { return now } },
    permissionSecondsLeft: { get value() { return permissionSecondsLeft } },
    permissionExpired: { get value() { return permissionExpired } },
    answerPermission,
  }
}
