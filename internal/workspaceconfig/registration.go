package workspaceconfig

import (
	"context"
	"time"
)

func registrationContext(base context.Context) (context.Context, context.CancelFunc) {
	if base == nil {
		base = context.Background()
	}
	return context.WithTimeout(base, 10*time.Second)
}
