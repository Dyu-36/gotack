package provider

import (
	"context"
	"strings"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

func SupportsVision(ctx context.Context, api *crushapi.Client, workspaceID, providerID, modelID string) (bool, error) {
	providers, err := api.ListProviders(ctx, workspaceID)
	if err != nil {
		return false, err
	}
	for _, candidate := range providers {
		if !strings.EqualFold(candidate.ID, providerID) {
			continue
		}
		for _, model := range candidate.Models {
			if strings.EqualFold(model.ID, modelID) {
				return model.SupportsVision, nil
			}
		}
	}
	return false, nil
}
