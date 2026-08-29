import { desktop, type ModelCatalogEntry, type ProviderCatalogEntry } from '../../platform/desktop'
import type { ReasoningEffort } from './types.svelte'

// catalog.svelte.ts -- role: the live provider and model catalog fetched from
// the Crush engine, plus the reasoning-effort vocabulary. Module-level state
// keeps Composer and SettingsModal in sync without prop drilling.

export const REASONING_EFFORT_OPTIONS: Array<{ id: ReasoningEffort; label: string; short: string }> = [
  { id: 'none', label: 'None (Không suy luận)', short: 'None' },
  { id: 'low', label: 'Low (Thấp)', short: 'Low' },
  { id: 'medium', label: 'Medium (Vừa)', short: 'Med' },
  { id: 'high', label: 'High (Sâu)', short: 'High' },
  { id: 'max', label: 'Max (Tối đa)', short: 'Max' },
]

type CatalogStatus = 'idle' | 'loading' | 'ready' | 'error'

let providers = $state<ProviderCatalogEntry[]>([])
let status = $state<CatalogStatus>('idle')
let loadError = $state('')

async function refresh() {
  if (status === 'loading') return
  status = 'loading'
  loadError = ''
  try {
    providers = await desktop.listProviders()
    if (!providers.length) throw new Error('Crush returned an empty provider catalog')
    status = 'ready'
  } catch (cause) {
    providers = []
    status = 'error'
    loadError = cause instanceof Error ? cause.message : String(cause)
  }
}

function reset() {
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
}
