# Conversation metadata

Assistant messages display the completion timestamp recorded in the engine's finish part. Older messages without a finish timestamp fall back to their last update, then creation time. User messages retain their creation time. Engine timestamps are normalized to milliseconds at the desktop boundary so reloading a conversation uses the same clock units as live messages.

The workspace stream includes session updates and forwards title changes to the conversation list, including when title generation finishes after the agent response. The engine remains responsible for saving generated titles.

Conversations whose stored title is empty or a default label use the first user message as their display title. This fallback is derived from saved history when listing older conversations and is shown immediately for a new prompt. It collapses whitespace and control characters, excludes extracted attachment contents, and limits the title to 80 Unicode code points. Attachment names are used when there is no text. Existing non-default names take precedence, and a rejected prompt restores its previous title.

This follows Pi's separation of saved session names and first-message display fallbacks: [session selector](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/src/modes/interactive/components/session-selector.ts), [session metadata](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/src/core/session-manager.ts). It does not change the engine's conversation storage format.

`TestProviderReasoning` now expects explicit `none` for disabled reasoning, matching the provider setting that prevents the engine from treating an empty effort as a request for its default.

Validation for this change consists of production builds, formatting, and Svelte type checking. No unit tests, integration tests, or model requests were run.
