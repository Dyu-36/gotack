import { describe, expect, it } from 'vitest'
import { nextStreamingText, streamingChunkSize, takeGraphemePrefix } from './streaming-text'

describe('streaming text pacing', () => {
  it('reveals a small backlog over more than one frame', () => {
    const next = nextStreamingText('', 'hello')
    expect(next).toBe('he')
  })

  it('catches up faster as the backlog grows', () => {
    expect(streamingChunkSize(4)).toBe(1)
    expect(streamingChunkSize(48)).toBe(5)
    expect(streamingChunkSize(500)).toBe(42)
  })

  it('applies a stream resync immediately', () => {
    expect(nextStreamingText('hello world', 'hello there')).toBe('hello there')
  })

  it('disables paced reveal for reduced motion', () => {
    expect(nextStreamingText('', 'complete', true)).toBe('complete')
  })

  it('does not split a grapheme cluster', () => {
    const family = '👨‍👩‍👧‍👦'
    expect(takeGraphemePrefix(`${family}x`, 1)).toBe(family)
  })
})
