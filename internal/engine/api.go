package engine

import (
	"context"

	"github.com/Dyu-36/gotack/internal/engineapi"
)

type EngineAPI interface {
	Owned() bool

	Locate(ctx context.Context) (engineapi.Endpoint, bool)

	Start() (engineapi.Endpoint, error)

	Stop() error
}
