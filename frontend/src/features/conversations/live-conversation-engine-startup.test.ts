import { afterEach, describe, expect, it, vi } from 'vitest'
import { createEngineState, type EngineDeps } from './live-conversation-engine.svelte'

const mocks = vi.hoisted(() => ({
  desktop: {
    backendReady: vi.fn(),
    attachmentLimits: vi.fn(),
    getSettings: vi.fn(),
    startEngine: vi.fn(),
    engineStatus: vi.fn(),
    listProviders: vi.fn(),
  },
  on: vi.fn(() => () => {}),
}))

vi.mock('../../platform/desktop', () => ({
  desktop: mocks.desktop,
  events: {
    engineStatus: 'engine:status',
    promptFiles: 'prompt:files',
    sessionDelta: 'session:delta',
    sessionDone: 'session:done',
    sessionUpdated: 'session:updated',
    toolActivity: 'tool:activity',
  },
  on: mocks.on,
}))

const starting = { status: 'starting' as const, running: false, endpoint: 'pipe', version: '', owned: true }
const running = { status: 'running' as const, running: true, endpoint: 'pipe', version: 'test', owned: true }

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
  vi.clearAllMocks()
})

describe('createEngineState startup', () => {
  it('keeps polling until a delayed engine becomes ready', async () => {
    vi.useFakeTimers()
    vi.stubGlobal('window', { setTimeout, clearTimeout })
    mocks.desktop.backendReady.mockResolvedValue(true)
    mocks.desktop.attachmentLimits.mockResolvedValue(null)
    mocks.desktop.getSettings.mockResolvedValue(null)
    mocks.desktop.startEngine.mockResolvedValue(starting)
    let statusCalls = 0
    mocks.desktop.engineStatus.mockImplementation(async () => {
      statusCalls += 1
      return statusCalls <= 24 ? starting : running
    })
    mocks.desktop.listProviders.mockResolvedValue([{
      id: 'test',
      name: 'Test',
      type: 'openai',
      models: [{ id: 'test-model', name: 'Test model', can_reason: false }],
      configured: true,
    }])

    const deps: EngineDeps = {
      conversations: { value: [] },
      backendReady: { value: false },
      engine: { value: null },
      error: { value: '' },
      streamingText: { value: '' },
      provider: { value: '' },
      model: { value: '' },
      modelLabel: { value: '' },
      thinking: { value: 'high' },
      apiKey: { value: '' },
      customUrl: { value: '' },
      activeId: { value: '' },
      reportError: vi.fn(),
      clearError: vi.fn(),
      updateConversation: vi.fn(),
      ensureWorkspace: vi.fn().mockResolvedValue(undefined),
      reloadMessages: vi.fn().mockResolvedValue(undefined),
      attachPaths: vi.fn(),
    }
    const engine = createEngineState(deps)
    const initialized = engine.init()
    await vi.advanceTimersByTimeAsync(7000)
    await initialized

    expect(statusCalls).toBeGreaterThan(24)
    expect(deps.backendReady.value).toBe(true)
    engine.destroy()
  })
})
