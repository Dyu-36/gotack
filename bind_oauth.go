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

// bind_oauth.go -- role: Wails-bound OAuth authentication methods for AI providers
// (starting with ChatGPT / OpenAI Codex PKCE flow).

// ChatGPTOAuthStatus represents the current OAuth connection status for ChatGPT.
type ChatGPTOAuthStatus struct {
	Connected bool   `json:"connected"`
	Email     string `json:"email,omitempty"`
	Plan      string `json:"plan,omitempty"`
	ExpiresAt int64  `json:"expires_at,omitempty"`
}

// LoginChatGPTOAuth launches the OAuth PKCE login flow for ChatGPT, opens the default
// browser for authentication, and persists the resulting tokens in the Crush engine.
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

	// Codex is not published by Catwalk, so the provider has to exist in config
	// before the engine will accept a credential for it. Seeding also enables it.
	if err := seedCodexProvider(a.ctx, svc.api, workspaceID, scope); err != nil {
		return ChatGPTOAuthStatus{}, err
	}

	// Store the OAuth token into Crush. The engine points the provider at the
	// Codex backend and stores the account-scoped model catalog alongside it.
	if err := svc.api.SetProviderOAuthToken(a.ctx, workspaceID, scope, codexProviderID, token); err != nil {
		return ChatGPTOAuthStatus{}, fmt.Errorf("save oauth token to engine: %w", err)
	}

	// Update local config preferences if needed
	if a.cfg == nil {
		a.cfg = appconfig.Defaults()
	}
	// A ChatGPT login only takes over the active provider when the user is not
	// deliberately on something else. "openai" counts as a previous ChatGPT
	// selection because it is where this credential used to live.
	if a.cfg.Provider == "" || a.cfg.Provider == openAIProviderID || a.cfg.Provider == codexProviderID {
		a.cfg.Provider = codexProviderID
		providers, listErr := svc.api.ListProviders(a.ctx, workspaceID)
		if listErr != nil {
			return ChatGPTOAuthStatus{}, fmt.Errorf("load ChatGPT subscription models: %w", listErr)
		}
		modelID, modelErr := selectChatGPTModel(providers, a.cfg.Model)
		if modelErr != nil {
			return ChatGPTOAuthStatus{}, modelErr
		}
		a.cfg.Model = modelID
		effort, think := crushReasoning(a.cfg.Thinking)
		selected := crushapi.SelectedModel{Provider: codexProviderID, Model: modelID, ReasoningEffort: effort, Think: think}
		if err := svc.api.SetPreferredModel(a.ctx, workspaceID, scope, "large", selected); err != nil {
			return ChatGPTOAuthStatus{}, fmt.Errorf("select ChatGPT large model: %w", err)
		}
		if err := svc.api.SetPreferredModel(a.ctx, workspaceID, scope, "small", selected); err != nil {
			return ChatGPTOAuthStatus{}, fmt.Errorf("select ChatGPT small model: %w", err)
		}
		cfgCopy := *a.cfg
		if err := appconfig.Save(&cfgCopy); err != nil {
			return ChatGPTOAuthStatus{}, fmt.Errorf("save ChatGPT model preference: %w", err)
		}
	}

	return ChatGPTOAuthStatus{
		Connected: true,
		Email:     token.UserEmail(),
		Plan:      token.UserPlan(),
		ExpiresAt: token.ExpiresAt,
	}, nil
}

// GetChatGPTOAuthStatus checks whether ChatGPT OAuth credentials are active in Crush.
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

	// Credentials written before "codex" existed still sit under "openai". They
	// are moved here so an upgraded install reports connected without the user
	// opening Settings or signing in again. A failure is not fatal: status is
	// still readable, and the next call retries.
	if moved, err := migrateChatGPTOAuthToCodex(ctx, svc.api, workspaceID); err != nil {
		a.warnCodexMigration("could not move the ChatGPT credential to the Codex provider", "err", err)
	} else if moved {
		// Whichever path moves the credential first owns the repoint. Without it
		// the saved selection keeps naming the provider that no longer holds a
		// credential, and the next settings replay restores the broken routing.
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

// LogoutChatGPTOAuth removes the ChatGPT OAuth credential and disables the
// Codex provider. An OpenAI API key stored separately is untouched.
func (a *App) LogoutChatGPTOAuth() error {
	return a.DeleteProvider(codexProviderID)
}
