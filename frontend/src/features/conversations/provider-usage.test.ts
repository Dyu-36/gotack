import { describe, expect, it } from 'vitest'
import { orderedUsageWindows, remainingPercent, usageWindowLabel } from './provider-usage'

const window = (seconds: number, remaining: number, name?: string) => ({
  id: `${seconds}`,
  name,
  used_percent: 100 - remaining,
  remaining_percent: remaining,
  window_seconds: seconds,
})

describe('provider usage presentation', () => {
  it('labels common provider-defined windows', () => {
    expect(usageWindowLabel(window(18_000, 75))).toBe('5h')
    expect(usageWindowLabel(window(604_800, 25))).toBe('Tuần')
    expect(usageWindowLabel(window(86_400, 50, 'Codex Spark'))).toBe('Codex Spark · 24h')
  })

  it('orders short windows before long windows', () => {
    const result = orderedUsageWindows([
      window(604_800, 25),
      window(18_000, 75),
      window(86_400, 50),
    ])
    expect(result.map((item) => item.window_seconds)).toEqual([18_000, 86_400, 604_800])
  })

  it('clamps rounded percentages for display', () => {
    expect(remainingPercent(window(18_000, 100.6))).toBe(100)
    expect(remainingPercent(window(18_000, -1))).toBe(0)
    expect(remainingPercent(window(18_000, 49.6))).toBe(50)
  })
})
