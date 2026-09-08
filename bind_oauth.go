package main

import (
	"context"
	"fmt"
	"os/exec"
	runtimeOS "runtime"
	"time"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/engineapi"
	"github.com/Dyu-36/gotack/internal/openaioauth"
	providerdomain "github.com/Dyu-36/gotack/internal/provider"
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
	if err := providerdomain.SeedCodex(a.ctx, svc.api, workspaceID, engineapi.ConfigScopeGlobal); err != nil {
		return ChatGPTOAuthStatus{}, err
	}
	if err := svc.api.SetProviderOAuthToken(a.ctx, workspaceID, engineapi.ConfigScopeGlobal, codexProviderID, token); err != nil {
		return ChatGPTOAuthStatus{}, fmt.Errorf("save oauth token to engine: %w", err)
	}

	current := a.cfg
	if current == nil {
		current = appconfig.Defaults()
	}
	if current.Provider == "" || current.Provider == openAIProviderID || current.Provider == codexProviderID {
		modelID, err := providerdomain.ApplyChatGPTLoginSelection(a.ctx, svc.api, workspaceID, current.Model, current.Thinking)
		if err != nil {
			return ChatGPTOAuthStatus{}, err
		}
		next := *current
		next.Provider = codexProviderID
		next.Model = modelID
		next.CustomURL = ""
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

	base := a.ctx
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithTimeout(base, 10*time.Second)
	defer cancel()
	if moved, err := providerdomain.MigrateChatGPTOAuthToCodex(ctx, svc.api, workspaceID); err != nil {
		a.warnCodexMigration("could not move the ChatGPT credential to the Codex provider", "err", err)
	} else if moved {
		a.repointSavedModelAtCodex(svc, workspaceID)
	}

	status, err := providerdomain.ChatGPTAuthStatus(ctx, svc.api, workspaceID, time.Now())
	if err != nil {
		return ChatGPTOAuthStatus{}, err
	}
	return ChatGPTOAuthStatus{
		Connected: status.Connected,
		Email:     status.Email,
		Plan:      status.Plan,
		ExpiresAt: status.ExpiresAt,
	}, nil
}

func (a *App) LogoutChatGPTOAuth() error {
	return a.DeleteProvider(codexProviderID)
}
