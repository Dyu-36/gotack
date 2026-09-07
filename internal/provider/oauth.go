package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/openaioauth"
)

type AuthStatus struct {
	Connected bool
	Email     string
	Plan      string
	ExpiresAt int64
}

func ChatGPTAuthStatus(ctx context.Context, api *crushapi.Client, workspaceID string, now time.Time) (AuthStatus, error) {
	cfg, err := api.GetWorkspaceConfig(ctx, workspaceID)
	if err != nil {
		return AuthStatus{}, fmt.Errorf("get Crush config: %w", err)
	}
	configured, ok := cfg.Providers[CodexID]
	if !ok || configured.Disable {
		return AuthStatus{Connected: false}, nil
	}
	rawOAuth := strings.TrimSpace(string(configured.OAuth))
	if rawOAuth == "" || rawOAuth == "null" || rawOAuth == "{}" {
		return AuthStatus{Connected: false}, nil
	}

	var token openaioauth.Token
	if err := json.Unmarshal(configured.OAuth, &token); err != nil || token.AccessToken == "" || token.AccountID == "" {
		return AuthStatus{Connected: false}, nil
	}
	if token.ExpiresAt > 0 && now.Unix() >= token.ExpiresAt {
		if token.RefreshToken == "" {
			return AuthStatus{Connected: false}, nil
		}
		if err := api.RefreshProviderOAuthToken(ctx, workspaceID, crushapi.ConfigScopeGlobal, CodexID); err != nil {
			return AuthStatus{Connected: false}, nil
		}
		cfg, err = api.GetWorkspaceConfig(ctx, workspaceID)
		if err != nil {
			return AuthStatus{}, fmt.Errorf("get refreshed Crush config: %w", err)
		}
		configured = cfg.Providers[CodexID]
		if err := json.Unmarshal(configured.OAuth, &token); err != nil || token.AccessToken == "" || token.AccountID == "" {
			return AuthStatus{Connected: false}, nil
		}
	}
	return AuthStatus{
		Connected: true,
		Email:     token.UserEmail(),
		Plan:      token.UserPlan(),
		ExpiresAt: token.ExpiresAt,
	}, nil
}
