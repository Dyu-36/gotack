package main

import (
	"context"
	"encoding/json"
	"fmt"

	"os/exec"
	runtimeOS "runtime"
	"strings"
	"time"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/openaioauth"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type ChatGPTOAuthStatus struct {
	Connected bool   `json:"connected"`
	Email     string `json:"email,omitempty"`
	Plan      string `json:"plan,omitempty"`
	ExpiresAt int64  `json:"expires_at,omitempty"`
}

func (a *App) LoginChatGPTOAuth() (ChatGPTOAuthStatus, error) {
	svc, err := a.services()
	if err != nil {
		return ChatGPTOAuthStatus{}, err
	}

	workspaceID, err := a.configWorkspaceID(a.ctx, svc)
	if err != nil {
		return ChatGPTOAuthStatus{}, err
	}

	opts := openaioauth.DefaultOptions()
	opts.OpenBrowser = func(authURL string) error {
		if a.ctx != nil {
			runtime.BrowserOpenURL(a.ctx, authURL)
			return nil
		}
		if runtimeOS.GOOS == "windows" {
			return exec.Command("rundll32", "url.dll,FileProtocolHandler", authURL).Start()
		}
		return exec.Command("xdg-open", authURL).Start()
	}

	token, err := openaioauth.StartLogin(a.ctx, opts)
	if err != nil {
		return ChatGPTOAuthStatus{}, fmt.Errorf("chatgpt oauth login failed: %w", err)
	}

	scope := crushapi.ConfigScopeGlobal

	if err := seedCodexProvider(a.ctx, svc.api, workspaceID, scope); err != nil {
		return ChatGPTOAuthStatus{}, err
	}

	if err := svc.api.SetProviderOAuthToken(a.ctx, workspaceID, scope, codexProviderID, token); err != nil {
		return ChatGPTOAuthStatus{}, fmt.Errorf("save oauth token to engine: %w", err)
	}

	current := a.cfg
	if current == nil {
		current = appconfig.Defaults()
	}

	if current.Provider == "" || current.Provider == openAIProviderID || current.Provider == codexProviderID {
		next := *current
		next.Provider = codexProviderID
		providers, listErr := svc.api.ListProviders(a.ctx, workspaceID)
		if listErr != nil {
			return ChatGPTOAuthStatus{}, fmt.Errorf("load ChatGPT subscription models: %w", listErr)
		}
		modelID, modelErr := selectChatGPTModel(providers, current.Model)
		if modelErr != nil {
			return ChatGPTOAuthStatus{}, modelErr
		}
		next.Model = modelID
		effort, think := crushReasoning(next.Thinking)
		selected := crushapi.SelectedModel{Provider: codexProviderID, Model: modelID, ReasoningEffort: effort, Think: think}
		if err := svc.api.SetPreferredModelPair(a.ctx, workspaceID, scope, selected); err != nil {
			return ChatGPTOAuthStatus{}, fmt.Errorf("select ChatGPT model: %w", err)
		}
		if err := appconfig.Save(&next); err != nil {
			return ChatGPTOAuthStatus{}, fmt.Errorf("save ChatGPT model preference: %w", err)
		}
		a.cfg = &next
	}

	return ChatGPTOAuthStatus{
		Connected: true,
		Email:     token.UserEmail(),
		Plan:      token.UserPlan(),
		ExpiresAt: token.ExpiresAt,
	}, nil
}

func (a *App) GetChatGPTOAuthStatus() (ChatGPTOAuthStatus, error) {
	svc, err := a.services()
	if err != nil {
		return ChatGPTOAuthStatus{}, err
	}

	workspaceID, err := a.configWorkspaceID(a.ctx, svc)
	if err != nil {
		return ChatGPTOAuthStatus{}, err
	}

	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()

	if moved, err := migrateChatGPTOAuthToCodex(ctx, svc.api, workspaceID); err != nil {
		a.warnCodexMigration("could not move the ChatGPT credential to the Codex provider", "err", err)
	} else if moved {

		a.repointSavedModelAtCodex(svc, workspaceID)
	}

	cfg, err := svc.api.GetWorkspaceConfig(ctx, workspaceID)
	if err != nil {
		return ChatGPTOAuthStatus{}, fmt.Errorf("get Crush config: %w", err)
	}

	pc, ok := cfg.Providers[codexProviderID]
	if !ok || pc.Disable {
		return ChatGPTOAuthStatus{Connected: false}, nil
	}

	rawOAuth := strings.TrimSpace(string(pc.OAuth))
	if rawOAuth == "" || rawOAuth == "null" || rawOAuth == "{}" {
		return ChatGPTOAuthStatus{Connected: false}, nil
	}

	var tok openaioauth.Token
	if err := json.Unmarshal(pc.OAuth, &tok); err != nil || tok.AccessToken == "" || tok.AccountID == "" {
		return ChatGPTOAuthStatus{Connected: false}, nil
	}
	if tok.ExpiresAt > 0 && time.Now().Unix() >= tok.ExpiresAt {
		if tok.RefreshToken == "" {
			return ChatGPTOAuthStatus{Connected: false}, nil
		}
		if err := svc.api.RefreshProviderOAuthToken(ctx, workspaceID, crushapi.ConfigScopeGlobal, codexProviderID); err != nil {
			return ChatGPTOAuthStatus{Connected: false}, nil
		}
		cfg, err = svc.api.GetWorkspaceConfig(ctx, workspaceID)
		if err != nil {
			return ChatGPTOAuthStatus{}, fmt.Errorf("get refreshed Crush config: %w", err)
		}
		pc = cfg.Providers[codexProviderID]
		if err := json.Unmarshal(pc.OAuth, &tok); err != nil || tok.AccessToken == "" || tok.AccountID == "" {
			return ChatGPTOAuthStatus{Connected: false}, nil
		}
	}
	return ChatGPTOAuthStatus{
		Connected: true,
		Email:     tok.UserEmail(),
		Plan:      tok.UserPlan(),
		ExpiresAt: tok.ExpiresAt,
	}, nil
}

func selectChatGPTModel(providers []crushapi.Provider, current string) (string, error) {
	for _, provider := range providers {
		if provider.ID != codexProviderID {
			continue
		}
		for _, model := range provider.Models {
			if current != "" && model.ID == current {
				return current, nil
			}
		}
		if provider.DefaultLargeModelID != "" {
			return provider.DefaultLargeModelID, nil
		}
		if len(provider.Models) > 0 {
			return provider.Models[0].ID, nil
		}
	}
	return "", fmt.Errorf("ChatGPT subscription returned no selectable models")
}

func (a *App) LogoutChatGPTOAuth() error {
	return a.DeleteProvider(codexProviderID)
}
