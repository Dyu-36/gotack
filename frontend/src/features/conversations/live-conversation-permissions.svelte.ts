import { desktop, type PermissionRequestPayload as Envelope, type QuestionRequestEvent } from '../../platform/desktop'

type QuestionAnswer = { request_id: string; selected_ids?: string[]; fill_in_text?: string; yes?: boolean | null }

export type PermissionDeps = {
  permission: { value: Envelope | null }
  question: { value: QuestionRequestEvent | null }
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

  // Permission TTL countdown: tick once per second while a permission is on
  // screen so the UI can render a live "expires in Ns" badge. Auto-clear the
  // modal a couple of seconds after expiry so the user sees the expired state
  // and the dialog disappears.
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

  const answerQuestion = async (answers: QuestionAnswer[]) => {
    const current = deps.question.value
    if (!current) return
    deps.question.value = null
    try { await desktop.answerQuestion(current.id, answers) } catch (cause) { deps.reportError(cause, 'Question response') }
  }

  return {
    now: { get value() { return now } },
    permissionSecondsLeft: { get value() { return permissionSecondsLeft } },
    permissionExpired: { get value() { return permissionExpired } },
    answerPermission,
    answerQuestion,
  }
}
