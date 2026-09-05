# Contract: OpenAI Reasoning Continuity

This document specifies the contract for preserving reasoning metadata across
multi-turn OpenAI Responses API interactions in Gotack/Crush.

## Scope

Ordered encrypted-only reasoning items must survive:
- Multiple turns within a session
- Engine restart
- Model/provider switch (with appropriate drop)
- Compaction (latest valid anchor preserved)

## Message Model

Each reasoning item carries:
- `item_id`: unique identifier within the session
- `encrypted_content`: provider-supplied opaque blob
- `provider`: model provider fingerprint
- `model`: model identifier at creation time
- `created_at`: timestamp
- `summary`: optional human-readable summary (not used for replay)

## Conversion Rules

### To Provider Request

Reasoning items are converted to structured Responses input items:

```json
{
  "type": "reasoning",
  "id": "<item_id>",
  "encrypted_content": "<opaque>"
}
```

Items appear in the request at their original relative position before the
related function call. Each `item_id` appears at most once per request.

### From Provider Response

When the provider returns reasoning content:
1. Create or update the reasoning item matching the `item_id`
2. Store encrypted content; do not decrypt or log plaintext
3. Preserve all metadata fields through start/delta/end lifecycle

## Replay Strategy

Use explicit local-history replay with `store=false`:
- All valid reasoning items from the session are replayed in order
- `previous_response_id` is NOT used when replaying reasoning history
- Reject configurations that combine `previous_response_id` with replayed
  reasoning items

## Model/Provider Switch

When the active model or provider changes:
- Drop reasoning items from incompatible providers
- Emit `prefix_changed_reason: "model_switch"` in telemetry
- Never log or hash ciphertext

## Compaction

After compaction:
- Keep ordered reasoning items of the latest valid assistant anchor
- Remove items that fell within the compacted range
- Emit compaction metadata in telemetry (tokens before/after, messages evicted)

## Forbidden Operations

- Never replay `Thinking` or `Summary[]` as assistant text
- Never log prompt text, ciphertext, OAuth tokens, or raw session UUIDs
- Never persist reasoning content to JSONL telemetry
- Never allow duplicate `item_id` in a single request (deterministic error)

## Backward Compatibility

- New Gotack reads old events without reasoning metadata gracefully
- New telemetry fields are optional (`omitempty`); old consumers skip them
- Provider request shape changes are gated behind feature flags
