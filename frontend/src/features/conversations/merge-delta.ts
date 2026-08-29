// merge-delta.ts -- role: pure helper that decides whether an incoming
// session:delta appends onto the previous text or forces a resync.
//
// Plain TypeScript (no Svelte runes) so the same module is exercised
// directly from Vitest without a Svelte runtime. The chat engine calls
// this from its sessionDelta handler; tests cover the merge logic in
// isolation.

export type DeltaState = { text: string; seq: number }

export type MergeResult =
  | { kind: 'ok'; text: string; seq: number }
  | { kind: 'resync'; text: string; seq: number }

// applyDelta folds an incoming session:delta onto the previous known
// state for a single message.
//
// Parameters:
//   prev     - last accepted delta for this message, or null if this
//              is the first delta the client has ever observed.
//   append   - the new suffix the server claims was added since the
//              last flush. The wire also carries the full `fullText`
//              snapshot so the caller can always rebuild.
//   seq      - the wire's monotonically increasing seq counter for
//              this message (starts at 1).
//   fullText - the wire's full text snapshot. On a resync the caller
//              MUST rebuild from this string; the server is the source
//              of truth, not the locally concatenated append.
//
// Returns either:
//   { kind: 'ok', ... }      - append onto prev.text, advance to seq.
//   { kind: 'resync', ... }  - rebuild from fullText because the seq
//                              chain is broken (gap, restart, or the
//                              first observed seq is not 1).
export function applyDelta(
  prev: DeltaState | null,
  append: string,
  seq: number,
  fullText: string,
): MergeResult {
  // No prior state: this is the first delta we have ever seen for
  // this message. The wire starts at seq=1; if it doesn't, treat it
  // as a restart and resync from the full snapshot.
  if (prev === null) {
    if (seq === 1) {
      return { kind: 'ok', text: fullText, seq }
    }
    return { kind: 'resync', text: fullText, seq }
  }

  // In-order continuation: prev.seq + 1 is exactly the next expected
  // seq. Append the suffix to the local view; the full snapshot is
  // unused because the chain is intact.
  if (seq === prev.seq + 1) {
    return { kind: 'ok', text: prev.text + append, seq }
  }

  // Anything else - gap, out-of-order delivery, restart of a stream
  // whose counter reset, or a duplicate - means the local view is no
  // longer authoritative. Rebuild from the server's full snapshot.
  return { kind: 'resync', text: fullText, seq }
}
