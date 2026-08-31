package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/zalo"
)

// bind_zalo.go -- role: Wails-bound API for the Zalo connection.
//
// The desktop host persists the channel state in <configDir>/zalo.json; the
// token is write-only across the Wails boundary. The runtime API surface
// mirrors the Stack one: pair a chat via /pair, send a file, regenerate the
// pairing code, and revoke a chat on demand.

// ZaloConfigInfo is the stored channel state returned to the UI; HasToken
// hides the secret instead of echoing it.
type ZaloConfigInfo struct {
	Enabled     bool     `json:"enabled"`
	PairedChats []string `json:"paired_chats"`
	PairingCode string   `json:"pairing_code"`
	HasToken    bool     `json:"has_token"`
	BotName     string   `json:"bot_name,omitempty"`
	TokenSuffix string   `json:"token_suffix,omitempty"`
	Running     bool     `json:"running"`
}

// ZaloConfigUpdate is the editable payload: an empty token keeps the stored
// one so the user can change settings without re-entering the secret.
type ZaloConfigUpdate struct {
	Enabled bool   `json:"enabled"`
	Token   string `json:"token,omitempty"`
}

// ZaloFileRequest sends a local file to a paired chat (or every paired chat
// when ChatID is empty).
type ZaloFileRequest struct {
	Path   string `json:"path"`
	ChatID string `json:"chat_id,omitempty"`
}

// ZaloManagerStatus is the live bridge snapshot returned to the UI.
type ZaloManagerStatus = zalo.Status

func (a *App) snapshotZaloConfig() ZaloConfigInfo {
	if a.zalo == nil {
		return ZaloConfigInfo{PairedChats: []string{}}
	}
	status := a.zalo.Status()
	return ZaloConfigInfo{
		Enabled:     a.cfg != nil && a.cfg.Zalo.Enabled,
		PairedChats: status.PairedChatIDs,
		PairingCode: status.PairingCode,
		HasToken:    status.Configured,
		BotName:     status.BotName,
		TokenSuffix: status.TokenSuffix,
		Running:     status.Running,
	}
}

// GetZaloConfig returns the current connection settings.
func (a *App) GetZaloConfig() ZaloConfigInfo {
	return a.snapshotZaloConfig()
}

// SaveZaloConfig persists the channel: validates the token, swaps the running
// bridge, and remembers the choice in the desktop config.
func (a *App) SaveZaloConfig(update ZaloConfigUpdate) (ZaloManagerStatus, error) {
	if a.zalo == nil {
		return ZaloManagerStatus{}, errors.New("zalo manager not initialised")
	}
	var status ZaloManagerStatus
	var err error
	if strings.TrimSpace(update.Token) != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		status, err = a.zalo.SetToken(ctx, strings.TrimSpace(update.Token))
		cancel()
		if err != nil {
			return status, err
		}
	} else if update.Enabled && !a.zalo.Status().Configured {
		return status, errors.New("bot token required to enable the Zalo connection")
	} else {
		status = a.zalo.Status()
	}
	if a.cfg == nil {
		a.cfg = appconfig.Defaults()
	}
	a.cfg.Zalo.Enabled = update.Enabled
	if err := appconfig.Save(a.cfg); err != nil {
		return status, err
	}
	if update.Enabled {
		a.zalo.Start()
	} else {
		a.zalo.Stop()
	}
	return a.zalo.Status(), nil
}

// TestZaloConnection validates the stored token and refreshes the bot name.
func (a *App) TestZaloConnection() (ZaloManagerStatus, error) {
	if a.zalo == nil {
		return ZaloManagerStatus{}, errors.New("zalo manager not initialised")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return a.zalo.TestConnection(ctx)
}

// RemoveZaloToken disconnects the bot and deletes all channel state.
func (a *App) RemoveZaloToken() (ZaloManagerStatus, error) {
	if a.zalo == nil {
		return ZaloManagerStatus{}, errors.New("zalo manager not initialised")
	}
	status, err := a.zalo.RemoveToken()
	if err != nil {
		return status, err
	}
	if a.cfg != nil {
		a.cfg.Zalo.Enabled = false
		a.cfg.Zalo.Token = ""
		a.cfg.Zalo.AllowedChats = nil
		_ = appconfig.Save(a.cfg)
	}
	return status, nil
}

// RegenerateZaloPairingCode rotates the displayed pairing code.
func (a *App) RegenerateZaloPairingCode() (ZaloManagerStatus, error) {
	if a.zalo == nil {
		return ZaloManagerStatus{}, errors.New("zalo manager not initialised")
	}
	return a.zalo.RegeneratePairingCode()
}

// UnpairZaloChat revokes a single paired chat and forgets its session.
func (a *App) UnpairZaloChat(chatID string) (ZaloManagerStatus, error) {
	if a.zalo == nil {
		return ZaloManagerStatus{}, errors.New("zalo manager not initialised")
	}
	return a.zalo.Unpair(chatID)
}

// ZaloStatus exposes the live bridge state for the UI.
func (a *App) ZaloStatus() ZaloManagerStatus {
	if a.zalo == nil {
		return ZaloManagerStatus{}
	}
	return a.zalo.Status()
}

// SendZaloFile pushes one local file to a paired chat from the desktop shell.
func (a *App) SendZaloFile(req ZaloFileRequest) (string, error) {
	if a.zalo == nil {
		return "", errors.New("zalo manager not initialised")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	return a.zalo.SendFile(ctx, req.Path, strings.TrimSpace(req.ChatID))
}
