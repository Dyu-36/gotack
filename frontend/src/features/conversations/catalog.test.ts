import { afterEach, describe, expect, it, vi } from 'vitest'
import { catalog } from './catalog.svelte'
import { desktop, type ProviderCatalogEntry } from '../../platform/desktop'

function provider(modelId: string): ProviderCatalogEntry {
  return {
    id: 'openai', name: 'OpenAI', configured: true,
    models: [{ id: modelId, name: modelId, can_reason: false, supports_vision: false }],
  }
}

afterEach(() => {
  catalog.reset()
  vi.restoreAllMocks()
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

describe('model catalog refresh', () => {
  it('automatically displays new models and stops polling on disconnect', async () => {
    vi.useFakeTimers()
    const list = vi.spyOn(desktop, 'listProviders')
      .mockResolvedValueOnce([provider('old')])
      .mockResolvedValue([provider('new')])
    await catalog.refresh()
    catalog.startAutoRefresh()
    await vi.advanceTimersByTimeAsync(5 * 60 * 1000)
    expect(list).toHaveBeenCalledTimes(2)
    expect(catalog.configuredModels.map((model) => model.id)).toEqual(['new'])
    catalog.reset()
    await vi.advanceTimersByTimeAsync(10 * 60 * 1000)
    expect(list).toHaveBeenCalledTimes(2)
    expect(catalog.providers).toEqual([])
  })

  it('retains the last usable catalog when a refresh fails', async () => {
    const list = vi.spyOn(desktop, 'listProviders').mockResolvedValue([provider('working')])
    await catalog.refresh()
    list.mockRejectedValueOnce(new Error('offline'))
    await catalog.refresh()
    expect(catalog.status).toBe('ready')
    expect(catalog.configuredModels[0].id).toBe('working')
    expect(catalog.error).toBe('offline')
    list.mockResolvedValueOnce([])
    await catalog.refresh()
    expect(catalog.configuredModels[0].id).toBe('working')
  })

  it('ignores a response from a disconnected engine', async () => {
    let resolve!: (providers: ProviderCatalogEntry[]) => void
    vi.spyOn(desktop, 'listProviders').mockImplementation(() => new Promise((done) => { resolve = done }))
    const loading = catalog.refresh()
    catalog.reset()
    resolve([provider('stale')])
    await loading
    expect(catalog.providers).toEqual([])
    expect(catalog.status).toBe('idle')
  })

  it('waits while the window is hidden and refreshes when visible again', async () => {
    vi.useFakeTimers()
    const page = { visibilityState: 'hidden' }
    vi.stubGlobal('document', page)
    const list = vi.spyOn(desktop, 'listProviders').mockResolvedValue([provider('model')])
    await catalog.refresh()
    catalog.startAutoRefresh()
    await vi.advanceTimersByTimeAsync(5 * 60 * 1000)
    expect(list).toHaveBeenCalledTimes(1)
    page.visibilityState = 'visible'
    await vi.advanceTimersByTimeAsync(5 * 60 * 1000)
    expect(list).toHaveBeenCalledTimes(2)
  })
})
