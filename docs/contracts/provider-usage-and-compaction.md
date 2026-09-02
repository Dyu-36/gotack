# Provider usage and proactive compaction

This document records the product and engine contract for the provider quota
badge and the 128K proactive context policy.

## Why this design

The provider-account quota and the current conversation context are different
signals:

- provider quota is an account-side rate limit such as a five-hour or weekly
  window;
- conversation context is the prompt-plus-completion content currently carried
  into the next model step.

The UI therefore must not present one as the other. Provider quota is shown as
the percentage returned by the provider. Context compaction remains an engine
policy driven by the session's live token count.

## Prior art reviewed

Before changing the pinned Crush engine, the implementation reviewed two coding
agents with explicit compaction policies:

- Pi enables auto-compaction by default, reserves output headroom, estimates
  trailing messages when exact usage is unavailable, summarizes at a safe
  message boundary, and keeps a configurable recent tail.
- OpenCode checks context against the model's usable input budget, prunes old
  tool output first, and then produces a summary while retaining a bounded
  recent tail.

Both approaches trigger before the advertised model window is completely full
and preserve recent working context. Gotack follows that principle without
replacing Crush's existing summary format or queue behavior.

## Provider usage contract

The only UI entry point is the Wails method:

```text
GetProviderUsage(providerID) -> ProviderUsageInfo
```

`ProviderUsageInfo` is provider-neutral:

```text
{
  provider_id,
  provider_name,
  available,
  plan?,
  limit_reached,
  windows: [{
    id,
    name?,
    used_percent,
    remaining_percent,
    window_seconds?,
    resets_at?
  }],
  updated_at,
  unavailable_reason?
}
```

Times are Unix milliseconds. The host owns all credentials and account-routing
headers; neither is sent to the webview.

### Adapters

- `codex`: reads the signed-in ChatGPT account's provider-defined primary,
  secondary, and additional rate-limit windows. A typical account exposes a
  five-hour primary window and a weekly secondary window, but the UI labels
  windows from `window_seconds` rather than assuming those two values.
- other providers: return `available: false` until the provider exposes a
  reliable account-usage endpoint or the engine supplies an equivalent signal.
  Gotack does not synthesize an absolute token balance from pricing, message
  count, or model context size.

The badge refreshes on backend readiness, selected-provider changes,
`session:done`, opening the popover, and explicit user refresh. It does not add a
polling loop.

## Proactive context policy

The engine patch computes:

```text
existing_safe_limit = context_window - existing_reserve
compact_at = min(existing_safe_limit, 128000)
```

The existing reserve is unchanged:

- context windows above 200K reserve 20K tokens;
- smaller context windows reserve 20 percent;
- an unknown context window (`0`) keeps auto-compaction disabled.

Examples:

| Model context | Compact at | Reason |
| ---: | ---: | --- |
| unknown | disabled | capacity cannot be inferred safely |
| 64K | 51,200 | existing 20% reserve |
| 128K | 102,400 | existing 20% reserve |
| 150K | 120,000 | existing 20% reserve |
| 200K | 128,000 | proactive cap |
| 1M | 128,000 | proactive cap |

The stop condition is evaluated after each completed agent step using the live
session prompt-plus-completion usage. Once the threshold is reached, Crush's
existing automatic summarization path runs and requeues the turn. The patch
therefore changes only *when* compaction starts, not the summary content,
session identity, tools, permissions, or REST/SSE contract.

## Acceptance checks

- provider response parsing preserves every provider-defined window and clamps
  percentages to `0..100`;
- ChatGPT requests carry bearer auth, `ChatGPT-Account-Id`, the Gotack user
  agent, and the FedRAMP routing header when required;
- unsupported providers report an explicit unavailable reason;
- Wails' exported method allowlist includes `GetProviderUsage`;
- frontend helpers label and order five-hour, daily, weekly, and additional
  windows;
- engine tests cover unknown, 64K, 128K, 150K, 200K, and 1M context sizes;
- the pinned Crush patch set still applies in filename order.
