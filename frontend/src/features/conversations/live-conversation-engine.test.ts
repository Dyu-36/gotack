import { afterEach, describe, expect, it, vi } from 'vitest'
import { desktop } from '../../platform/desktop'
import { catalog } from './catalog.svelte'
import { createEngineState, type EngineDeps } from './live-conversation-engine.svelte'

afterEach(() => {
  catalog.reset()
  vi.restoreAllMocks()
})

describe('createEngineState model selection', () => {
  it('does not submit the engine-owned Codex endpoint as a custom URL', async () => {
    vi.spyOn(desktop, 'listProviders').mockResolvedValue([
      {
        id: 'codex',
        name: 'ChatGPT (Codex)',
        type: 'openai',
        api_endpoint: 'https://chatgpt.com/backend-api/codex',
        models: [{ id: 'gpt-5-codex', name: 'GPT-5 Codex', can_reason: true }],
        configured: true,
        credential_kind: 'oauth',
      },
    ])
    const saveSettings = vi.spyOn(desktop, 'saveSettings').mockResolvedValue(undefined)
    await catalog.refresh()

    const deps: EngineDeps = {
      conversations: { value: [] },
      backendReady: { value: true },
      engine: { value: null },
      error: { value: '' },
      permission: { value: null },
      streamingText: { value: '' },
      provider: { value: 'openai' },
      model: { value: 'gpt-4o' },
      modelLabel: { value: 'GPT-4o' },
      thinking: { value: 'high' },
      apiKey: { value: '' },
      customUrl: { value: 'https://api.openai.com/v1' },
      activeId: { value: '' },
      reportError: vi.fn(),
      clearError: vi.fn(),
      updateConversation: vi.fn(),
      ensureWorkspace: vi.fn().mockResolvedValue(undefined),
      reloadMessages: vi.fn().mockResolvedValue(undefined),
      attachPaths: vi.fn(),
    }
    const engine = createEngineState(deps)

    engine.setModel('gpt-5-codex', 'GPT-5 Codex', 'codex')
    expect(await engine.waitForSelection()).toBe(true)

    expect(saveSettings).toHaveBeenCalledWith({
      theme: '',
      provider: 'codex',
      model: 'gpt-5-codex',
      thinking: 'high',
      api_key: '',
      custom_url: '',
    })
    expect(deps.customUrl.value).toBe('')
    expect(deps.error.value).toBe('')
  })
})
