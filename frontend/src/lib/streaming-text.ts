type GraphemeSegment = { index: number; segment: string }
type GraphemeSegmenter = { segment(input: string): Iterable<GraphemeSegment> }
type GraphemeSegmenterConstructor = new (
  locales?: string | string[],
  options?: { granularity?: 'grapheme' },
) => GraphemeSegmenter

const Segmenter = (Intl as unknown as { Segmenter?: GraphemeSegmenterConstructor }).Segmenter
const graphemeSegmenter = Segmenter ? new Segmenter(undefined, { granularity: 'grapheme' }) : null

export function streamingChunkSize(backlog: number): number {
  const size = Math.max(0, Math.floor(backlog))
  if (size === 0) return 0
  if (size <= 4) return 1
  if (size <= 12) return 2
  if (size <= 24) return 3
  if (size <= 48) return 5
  if (size <= 96) return 8
  if (size <= 192) return 14
  return Math.min(96, Math.max(24, Math.ceil(size / 12)))
}

export function takeGraphemePrefix(value: string, count: number): string {
  if (!value || count <= 0) return ''
  if (!graphemeSegmenter) return Array.from(value).slice(0, count).join('')

  let end = 0
  let seen = 0
  for (const part of graphemeSegmenter.segment(value)) {
    end = part.index + part.segment.length
    seen += 1
    if (seen >= count) break
  }
  return value.slice(0, end)
}

/**
 * Advances displayed text towards the latest stream snapshot without adding a
 * fixed delay. Small backlogs reveal one or two graphemes per animation frame;
 * larger bursts catch up aggressively so the UI never drifts far behind the
 * engine. A resync is applied immediately because the existing prefix is no
 * longer trustworthy.
 */
export function nextStreamingText(current: string, target: string, reducedMotion = false): string {
  if (reducedMotion || !target.startsWith(current)) return target

  const pending = target.slice(current.length)
  if (!pending) return current

  return current + takeGraphemePrefix(pending, streamingChunkSize(pending.length))
}
