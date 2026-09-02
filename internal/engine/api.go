package engine

import (
	"context"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

type EngineAPI interface {
	Owned() bool

	Locate(ctx context.Context) (crushapi.Endpoint, bool)

	Start() (crushapi.Endpoint, error)

	Stop() error
}
