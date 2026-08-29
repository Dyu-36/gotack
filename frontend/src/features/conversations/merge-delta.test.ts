// merge-delta.test.ts -- role: Vitest unit tests for applyDelta.
//
// The helper is the only seam between the wire session:delta event and
// the live ChatMessage state, so it must preserve five properties:
//
//   1. First delta at seq=1 anchors the text to the wire's fullText.
//   2. Subsequent deltas at seq=prev.seq+1 append the suffix.
//   3. A seq gap (drop, restart, or out-of-order) forces a resync from
//      the full snapshot; the local concatenated view is discarded.
//   4. The wire treats seq=0 as a resync sentinel: even with prior
//      state we drop the local view and rebuild from fullText.
//   5. A first delta with seq>1 (e.g. the client joins mid-stream)
//      also resyncs rather than producing a half-built message.

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
    // seq=3 is missing 2; the wire has dropped a frame.
    const result = applyDelta(prev, ' world!', 3, 'Hello world!')
    expect(result.kind).toBe('resync')
    expect(result).toEqual({ kind: 'resync', text: 'Hello world!', seq: 3 })
  })

  it('resyncs on a restart where seq resets to 1 mid-message', () => {
    const prev = { text: 'Hello', seq: 5 }
    // Engine restarted, counter begins again at 1.
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
    // Late-joining client, wire has already emitted seq=2.
    const result = applyDelta(null, ' world', 2, 'Hello world')
    expect(result.kind).toBe('resync')
    expect(result).toEqual({ kind: 'resync', text: 'Hello world', seq: 2 })
  })

  it('a resync followed by an in-order append rebuilds correctly', () => {
    // First delta forces a resync (seq 2 after seq 5 is a gap/restart); next delta at seq=3 appends.
    const r1 = applyDelta({ text: 'stale', seq: 5 }, '!', 2, 'Hi!')
    expect(r1.kind).toBe('resync')
    const state = { text: r1.text, seq: r1.seq }

    const r2 = applyDelta(state, '!', 3, 'Hi!!')
    expect(r2.kind).toBe('ok')
    expect(r2.text).toBe('Hi!!')
    expect(r2.seq).toBe(3)
  })

  it('append is unused on resync; fullText always wins', () => {
    // Even if append claims a large suffix, the resync returns fullText.
    const prev = { text: 'old', seq: 1 }
    const result = applyDelta(prev, 'long-suffix-to-ignore', 10, 'snapshot')
    expect(result.kind).toBe('resync')
    expect(result.text).toBe('snapshot')
  })
})
