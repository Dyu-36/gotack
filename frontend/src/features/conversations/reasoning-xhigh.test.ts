import { afterEach, describe, expect, it, vi } from 'vitest'
import { desktop } from '../../platform/desktop'
import { catalog, REASONING_EFFORT_OPTIONS } from './catalog.svelte'
import { createEngineState, type EngineDeps } from './live-conversation-engine.svelte'
import type { ReasoningEffort } from './types.svelte'

function engineDeps(): EngineDeps {
  return {
    conversations: { value: [] },
    backendReady: { value: true },
    engine: { value: null },
    error: { value: '' },
    permission: { value: null },
    question: { value: null },
    streamingText: { value: '' },
    provider: { value: 'openai' },
    model: { value: 'gpt-old' },
    modelLabel: { value: 'GPT old' },
    thinking: { value: 'max' },
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
}

afterEach(() => {
  catalog.reset()
  vi.restoreAllMocks()
})

describe('extended reasoning compatibility', () => {
  it('exposes minimal and xhigh in the picker vocabulary', () => {
    expect(REASONING_EFFORT_OPTIONS).toEqual(expect.arrayContaining([
      { id: 'minimal', label: 'Minimal (Tối thiểu)', short: 'Min' },
      { id: 'xhigh', label: 'X-High (Rất sâu)', short: 'X-High' },
    ]))
  })

  it.each<{
    effort: ReasoningEffort
    levels: ReasoningEffort[]
  }>([
    { effort: 'minimal', levels: ['none', 'minimal', 'low', 'high'] },
    { effort: 'xhigh', levels: ['low', 'medium', 'high', 'xhigh'] },
  ])('normalizes an incompatible stored level to $effort and persists it', async ({ effort, levels }) => {
    vi.spyOn(desktop, 'listProviders').mockResolvedValue([
      {
        id: 'codex',
        name: 'ChatGPT (Codex)',
        type: 'openai',
        api_endpoint: 'https://chatgpt.com/backend-api/codex',
        models: [{
          id: 'gpt-5-codex',
          name: 'GPT-5 Codex',
          can_reason: true,
          reasoning_levels: levels,
          default_reasoning_effort: effort,
        }],
        configured: true,
        credential_kind: 'oauth',
      },
    ])
    const saveSettings = vi.spyOn(desktop, 'saveSettings').mockResolvedValue(undefined)
    await catalog.refresh()

    const deps = engineDeps()
    const engine = createEngineState(deps)
    engine.setModel('gpt-5-codex', 'GPT-5 Codex', 'codex')

    expect(await engine.waitForSelection()).toBe(true)
    expect(deps.thinking.value).toBe(effort)
    expect(saveSettings).toHaveBeenCalledWith({
      theme: '',
      provider: 'codex',
      model: 'gpt-5-codex',
      thinking: effort,
      api_key: '',
      custom_url: '',
    })
  })
})
