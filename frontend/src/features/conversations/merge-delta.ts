
export type DeltaState = { text: string; seq: number }

export type MergeResult =
  | { kind: 'ok'; text: string; seq: number }
  | { kind: 'resync'; text: string; seq: number }

export function applyDelta(
  prev: DeltaState | null,
  append: string,
  seq: number,
  fullText: string,
): MergeResult {

  if (prev === null) {
    if (seq === 1) {
      return { kind: 'ok', text: fullText, seq }
    }
    return { kind: 'resync', text: fullText, seq }
  }

  if (seq === prev.seq) {
    if (fullText === prev.text) return { kind: 'ok', text: prev.text, seq: prev.seq }
    return { kind: 'resync', text: fullText, seq }
  }

  if (seq < prev.seq) {
    if (prev.text.startsWith(fullText)) return { kind: 'ok', text: prev.text, seq: prev.seq }
    return { kind: 'resync', text: fullText, seq }
  }

  if (seq === prev.seq + 1) {
    const text = prev.text + append
    if (text === fullText) {
      return { kind: 'ok', text, seq }
    }
    return { kind: 'resync', text: fullText, seq }
  }

  return { kind: 'resync', text: fullText, seq }
}
