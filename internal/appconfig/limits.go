package appconfig

import "time"

const (
	MaxAttachmentBytes = 5 * 1024 * 1024

	MaxDerivedLines = 2000
	MaxDerivedBytes = 500 * 1024
)

const (
	AttachmentCacheTTL = 14 * 24 * time.Hour

	AttachmentCacheBudget int64 = 512 * 1024 * 1024
)
