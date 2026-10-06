import type { ProviderUsageWindow } from '../../platform/desktop'
import { localeTag, t } from '../../lib/i18n.svelte'

export function orderedUsageWindows(windows: ProviderUsageWindow[]): ProviderUsageWindow[] {
  return [...windows].sort((left, right) => {
    const leftSeconds = left.window_seconds ?? Number.MAX_SAFE_INTEGER
    const rightSeconds = right.window_seconds ?? Number.MAX_SAFE_INTEGER
    if (leftSeconds !== rightSeconds) return leftSeconds - rightSeconds
    return (left.name ?? '').localeCompare(right.name ?? '')
  })
}

export function usageWindowLabel(window: ProviderUsageWindow): string {
  const seconds = window.window_seconds ?? 0
  let period = t('usage.period.session')
  if (seconds >= 4 * 3600 && seconds <= 6 * 3600) period = '5h'
  else if (seconds >= 20 * 3600 && seconds <= 28 * 3600) period = '24h'
  else if (seconds >= 6 * 86400 && seconds <= 8 * 86400) period = t('usage.period.week')
  else if (seconds > 0 && seconds % 86400 === 0) period = t('usage.period.days', { n: seconds / 86400 })
  else if (seconds > 0 && seconds % 3600 === 0) period = `${seconds / 3600}h`

  const name = (window.name ?? '').trim()
  return name ? `${name} · ${period}` : period
}

export function remainingPercent(window: ProviderUsageWindow): number {
  return Math.max(0, Math.min(100, Math.round(window.remaining_percent)))
}

export function formatUsageReset(resetAt?: number): string {
  if (!resetAt) return t('usage.noReset')
  return new Intl.DateTimeFormat(localeTag(), {
    day: '2-digit',
    month: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(resetAt))
}
