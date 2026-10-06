import { desktop, type ModelCatalogEntry, type ProviderCatalogEntry } from '../../platform/desktop'
import type { ReasoningEffort } from './types.svelte'

export const REASONING_EFFORT_OPTIONS: Array<{ id: ReasoningEffort; labelKey: string; short: string }> = [
  { id: 'none', labelKey: 'reasoning.none', short: 'None' },
  { id: 'minimal', labelKey: 'reasoning.minimal', short: 'Min' },
  { id: 'low', labelKey: 'reasoning.low', short: 'Low' },
  { id: 'medium', labelKey: 'reasoning.medium', short: 'Med' },
  { id: 'high', labelKey: 'reasoning.high', short: 'High' },
  { id: 'xhigh', labelKey: 'reasoning.xhigh', short: 'X-High' },
  { id: 'max', labelKey: 'reasoning.max', short: 'Max' },
]

type CatalogStatus = 'idle' | 'loading' | 'ready' | 'error'

let providers = $state<ProviderCatalogEntry[]>([])
let status = $state<CatalogStatus>('idle')
let loadError = $state('')
let refreshEpoch = 0
let refreshPromise: Promise<void> | null = null
const REFRESH_INTERVAL_MS = 5 * 60 * 1000
let autoRefresh = false
let refreshTimer: ReturnType<typeof setTimeout> | null = null

function clearRefreshTimer() {
  if (refreshTimer !== null) clearTimeout(refreshTimer)
  refreshTimer = null
}

function scheduleRefresh() {
  clearRefreshTimer()
  if (!autoRefresh) return
  refreshTimer = setTimeout(async () => {
    refreshTimer = null
    if (typeof document === 'undefined' || document.visibilityState !== 'hidden') await refresh()
    scheduleRefresh()
  }, REFRESH_INTERVAL_MS)
}

function startAutoRefresh() {
  autoRefresh = true
  scheduleRefresh()
}

function stopAutoRefresh() {
  autoRefresh = false
  clearRefreshTimer()
}

async function refresh() {
  if (refreshPromise) return refreshPromise
  const epoch = refreshEpoch
  if (!providers.length) status = 'loading'
  loadError = ''
  refreshPromise = (async () => {
    try {
      const rawProviders = await desktop.listProviders()
      if (epoch !== refreshEpoch) return
      if (!rawProviders.length) throw new Error('Backend returned an empty provider catalog')
      providers = rawProviders
        .map((p) => ({
          ...p,
          models: p.models || [],
        }))
        .sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }))
      status = 'ready'
    } catch (cause) {
      if (epoch !== refreshEpoch) return
      status = providers.length ? 'ready' : 'error'
      loadError = cause instanceof Error ? cause.message : String(cause)
    } finally {
      if (epoch === refreshEpoch) refreshPromise = null
    }
  })()
  return refreshPromise
}

function reset() {
  stopAutoRefresh()
  refreshEpoch += 1
  refreshPromise = null
  providers = []
  status = 'idle'
  loadError = ''
}

export const catalog = {
  get providers(): ProviderCatalogEntry[] {
    return providers
  },
  get status(): CatalogStatus {
    return status
  },
  get error(): string {
    return loadError
  },
  get models(): Array<ModelCatalogEntry & { providerId: string }> {
    return providers.flatMap((provider) => provider.models.map((model) => ({ ...model, providerId: provider.id })))
  },
  get configuredProviders(): ProviderCatalogEntry[] {
    return providers.filter((provider) => provider.configured)
  },
  get configuredModels(): Array<ModelCatalogEntry & { providerId: string }> {
    return providers
      .filter((provider) => provider.configured)
      .flatMap((provider) => provider.models.map((model) => ({ ...model, providerId: provider.id })))
  },
  provider(id: string): ProviderCatalogEntry | undefined {
    return providers.find((provider) => provider.id === id)
  },
  modelName(modelID: string, providerID?: string): string | undefined {
    const match = providers
      .filter((provider) => !providerID || provider.id === providerID)
      .flatMap((provider) => provider.models)
      .find((model) => model.id === modelID)
    return match?.name
  },
  refresh,
  reset,
  startAutoRefresh,
  stopAutoRefresh,
}
