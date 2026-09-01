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

	// Store OAuth token into Crush
	if err := svc.api.SetProviderOAuthToken(a.ctx, workspaceID, scope, "openai", token); err != nil {
		return ChatGPTOAuthStatus{}, fmt.Errorf("save oauth token to engine: %w", err)
	}

	// Enable provider in Crush
	if err := svc.api.SetConfigField(a.ctx, workspaceID, scope, "providers.openai.disable", false); err != nil {
		return ChatGPTOAuthStatus{}, fmt.Errorf("enable openai provider: %w", err)
	}

	// Update local config preferences if needed
	if a.cfg == nil {
		a.cfg = appconfig.Defaults()
	}
	if a.cfg.Provider == "" || a.cfg.Provider == "openai" {
		a.cfg.Provider = "openai"
		if a.cfg.Model == "" {
			a.cfg.Model = "gpt-4o"
		}
		_ = a.applyCrushSettings(SettingsInfo{
			Theme:    a.cfg.Theme,
			Provider: a.cfg.Provider,
			Model:    a.cfg.Model,
			Thinking: a.cfg.Thinking,
		}, "")
		cfgCopy := *a.cfg
		_ = appconfig.Save(&cfgCopy)
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

	cfg, err := svc.api.GetWorkspaceConfig(ctx, workspaceID)
	if err != nil {
		return ChatGPTOAuthStatus{}, fmt.Errorf("get Crush config: %w", err)
	}

	pc, ok := cfg.Providers["openai"]
	if !ok || pc.Disable {
		return ChatGPTOAuthStatus{Connected: false}, nil
	}

	rawOAuth := strings.TrimSpace(string(pc.OAuth))
	if rawOAuth == "" || rawOAuth == "null" || rawOAuth == "{}" {
		return ChatGPTOAuthStatus{Connected: false}, nil
	}

	var tok openaioauth.Token
	if err := json.Unmarshal(pc.OAuth, &tok); err == nil && tok.AccessToken != "" {
		return ChatGPTOAuthStatus{
			Connected: true,
			Email:     tok.UserEmail(),
			Plan:      tok.UserPlan(),
			ExpiresAt: tok.ExpiresAt,
		}, nil
	}


	return ChatGPTOAuthStatus{Connected: true}, nil
}

// LogoutChatGPTOAuth removes ChatGPT OAuth credentials and disables the OpenAI provider.
func (a *App) LogoutChatGPTOAuth() error {
	return a.DeleteProvider("openai")
}

