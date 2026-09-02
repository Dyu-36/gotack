
import { describe, expect, it } from 'vitest'
import { applyDelta } from './merge-delta'

describe('applyDelta', () => {
  it('anchors the first delta to the wire fullText when seq=1', () => {
    const result = applyDelta(null, 'Hello', 1, 'Hello')
    expect(result).toEqual({ kind: 'ok', text: 'Hello', seq: 1 })
  })

  it('appends the suffix when seq is exactly prev.seq + 1', () => {
    const prev = { text: 'Hello', seq: 1 }
    const result = applyDelta(prev, ' world', 2, 'Hello world')
    expect(result).toEqual({ kind: 'ok', text: 'Hello world', seq: 2 })
  })

  it('keeps appending across several in-order flushes', () => {
    let state: { text: string; seq: number } | null = null
    const frames = [
      { seq: 1, append: 'Hello', text: 'Hello' },
      { seq: 2, append: ' world', text: 'Hello world' },
      { seq: 3, append: '!', text: 'Hello world!' },
    ]
    for (const f of frames) {
      const r = applyDelta(state, f.append, f.seq, f.text)
      expect(r.kind).toBe('ok')
      state = { text: r.text, seq: r.seq }
    }
    expect(state).toEqual({ text: 'Hello world!', seq: 3 })
  })

  it('resyncs when the incoming seq is greater than prev.seq + 1', () => {
    const prev = { text: 'Hello', seq: 1 }

    const result = applyDelta(prev, ' world!', 3, 'Hello world!')
    expect(result.kind).toBe('resync')
    expect(result).toEqual({ kind: 'resync', text: 'Hello world!', seq: 3 })
  })

  it('resyncs on a restart where seq resets to 1 mid-message', () => {
    const prev = { text: 'Hello', seq: 5 }

    const result = applyDelta(prev, 'Hi', 1, 'Hi')
    expect(result.kind).toBe('resync')
    expect(result).toEqual({ kind: 'resync', text: 'Hi', seq: 1 })
  })

  it('resyncs on an out-of-order (lower) seq', () => {
    const prev = { text: 'Hello', seq: 3 }
    const result = applyDelta(prev, 'lo', 2, 'Hello')
    expect(result.kind).toBe('resync')
    expect(result.text).toBe('Hello')
  })

  it('treats seq=0 as a resync even with prior state', () => {
    const prev = { text: 'Hello', seq: 1 }
    const result = applyDelta(prev, '', 0, 'Fresh start')
    expect(result.kind).toBe('resync')
    expect(result).toEqual({ kind: 'resync', text: 'Fresh start', seq: 0 })
  })

  it('resyncs on the first ever delta when seq is not 1', () => {

    const result = applyDelta(null, ' world', 2, 'Hello world')
    expect(result.kind).toBe('resync')
    expect(result).toEqual({ kind: 'resync', text: 'Hello world', seq: 2 })
  })

  it('a resync followed by an in-order append rebuilds correctly', () => {

    const r1 = applyDelta({ text: 'stale', seq: 5 }, '!', 2, 'Hi!')
    expect(r1.kind).toBe('resync')
    const state = { text: r1.text, seq: r1.seq }

    const r2 = applyDelta(state, '!', 3, 'Hi!!')
    expect(r2.kind).toBe('ok')
    expect(r2.text).toBe('Hi!!')
    expect(r2.seq).toBe(3)
  })

  it('append is unused on resync; fullText always wins', () => {

    const prev = { text: 'old', seq: 1 }
    const result = applyDelta(prev, 'long-suffix-to-ignore', 10, 'snapshot')
    expect(result.kind).toBe('resync')
    expect(result.text).toBe('snapshot')
  })
})
