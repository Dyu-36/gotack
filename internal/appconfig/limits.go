package appconfig

import "time"

// limits.go -- role: one source of truth for attachment limits.
//
// The host, internal/attachments and the webview must agree on these numbers.
// The UI reads them through App.AttachmentLimits() instead of hardcoding 5 MB a
// second time, which is how the composer and the host drifted apart before.

const (
	// MaxAttachmentBytes caps a single uploaded, dropped or tagged file.
	MaxAttachmentBytes = 5 * 1024 * 1024

	// MaxDerivedLines and MaxDerivedBytes cap the extracted text inlined into
	// a prompt. Bigger files still reach the agent through their file path.
	MaxDerivedLines = 2000
	MaxDerivedBytes = 500 * 1024
)

// Attachment cache retention. PruneCache runs once per launch, so these are
// budgets, not schedules: no background loop watches the directory.
const (
	// AttachmentCacheTTL drops cache entries untouched for two weeks.
	AttachmentCacheTTL = 14 * 24 * time.Hour

	// AttachmentCacheBudget caps the total size of the attachment cache.
	AttachmentCacheBudget int64 = 512 * 1024 * 1024
)
