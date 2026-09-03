import { describe, expect, it, vi } from 'vitest'

vi.mock('dompurify', () => ({
  default: {
    sanitize: (html: string) => html,
  },
}))

import { localFilePath, renderMarkdownBlocks } from './markdown'

describe('streaming markdown blocks', () => {
  it('reuses settled block objects while the tail grows', () => {
    let sequence = 0
    const createId = () => `block-${++sequence}`
    const first = renderMarkdownBlocks('First paragraph.\n\nSec', [], createId)
    const second = renderMarkdownBlocks('First paragraph.\n\nSecond paragraph.', first, createId)

    expect(second).toHaveLength(2)
    expect(second[0]).toBe(first[0])
    expect(second[1]).not.toBe(first[1])
    expect(second[1]?.id).toBe(first[1]?.id)
  })

  it('keeps a fenced code block together while it streams', () => {
    const blocks = renderMarkdownBlocks('```ts\nconst value = 1')
    expect(blocks).toHaveLength(1)
    expect(blocks[0]?.type).toBe('code')
  })

  it('retains reference definitions when rendering blocks independently', () => {
    const blocks = renderMarkdownBlocks('[Example][docs]\n\n[docs]: https://example.com')
    expect(blocks[0]?.html).toContain('href="https://example.com"')
  })

  it('recognizes clickable generated file targets', () => {
    expect(localFilePath('C:\\Users\\Admin\\kết quả.xlsx')).toBe('C:\\Users\\Admin\\kết quả.xlsx')
    expect(localFilePath('file:///C:/Users/Admin/k%E1%BA%BFt%20qu%E1%BA%A3.xlsx')).toBe('C:\\Users\\Admin\\kết quả.xlsx')
    expect(localFilePath('https://example.com/report.xlsx')).toBeUndefined()
    expect(localFilePath('C:\\temp\\run.exe')).toBeUndefined()
  })
})
