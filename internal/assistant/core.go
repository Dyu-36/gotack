// Package assistant contains the product-owned, immutable assistant identity.
package assistant

import _ "embed"

// CorePrompt is shipped inside the executable, never loaded from user folders.
//
//go:embed core.md
var CorePrompt string
