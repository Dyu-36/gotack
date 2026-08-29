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
// The bridge polls the official Zalo Bot API and relays allowed chats to the
// agent; answers travel back over sendMessage when an agent run completes.

// ZaloConfigInfo is the stored Zalo connection. The token is write-only:
// HasToken tells the UI whether one is stored without echoing the secret.
type ZaloConfigInfo struct {
	Enabled      bool     `json:"enabled"`
	AllowedChats []string `json:"allowed_chats"`
	HasToken     bool     `json:"has_token"`
}

// ZaloConfigUpdate is the editable payload for SaveZaloConfig. An empty
// token keeps the stored one so the UI can change settings without
// re-entering the secret.
type ZaloConfigUpdate struct {
	Enabled      bool     `json:"enabled"`
	Token        string   `json:"token,omitempty"`
	AllowedChats []string `json:"allowed_chats"`
}

// GetZaloConfig returns the current Zalo connection settings.
func (a *App) GetZaloConfig() ZaloConfigInfo {
	if a.cfg == nil {
		return ZaloConfigInfo{AllowedChats: []string{}}
	}
	chats := a.cfg.Zalo.AllowedChats
	if chats == nil {
		chats = []string{}
	}
	return ZaloConfigInfo{
		Enabled:      a.cfg.Zalo.Enabled,
		AllowedChats: chats,
		HasToken:     strings.TrimSpace(a.cfg.Zalo.Token) != "",
	}
}

// SaveZaloConfig persists the connection, validates the bot token with getMe,
// and restarts the bridge to match the new configuration.
func (a *App) SaveZaloConfig(update ZaloConfigUpdate) (zalo.Status, error) {
	stored := ""
	if a.cfg != nil {
		stored = a.cfg.Zalo.Token
	}

	token := stored
	if strings.TrimSpace(update.Token) != "" {
		token = strings.TrimSpace(update.Token)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		bot, err := zalo.NewClient(token).GetMe(ctx)
		cancel()
		if err != nil {
			return zalo.Status{}, err
		}
		a.log.Info("zalo token validated", "bot", bot.Name)
	}
	if update.Enabled && strings.TrimSpace(token) == "" {
		return zalo.Status{}, errors.New("bot token required to enable the Zalo connection")
	}

	chats := make([]string, 0, len(update.AllowedChats))
	for _, id := range update.AllowedChats {
		if id = strings.TrimSpace(id); id != "" {
			chats = append(chats, id)
		}
	}

	if a.cfg == nil {
		a.cfg = appconfig.Defaults()
	}
	a.cfg.Zalo.Enabled = update.Enabled
	a.cfg.Zalo.Token = token
	a.cfg.Zalo.AllowedChats = chats
	cfgCopy := *a.cfg

	a.stopZaloBridge()
	if update.Enabled && a.EngineStatus().Running {
		a.startZaloBridge()
	}
	if err := appconfig.Save(&cfgCopy); err != nil {
		return zalo.Status{}, err
	}
	return a.ZaloStatus(), nil
}

// ZaloStatus reports bridge health, the bot identity, and the most recent
// inbound message so the user can copy a chat id into the allow list.
func (a *App) ZaloStatus() zalo.Status {
	c := a.getConn()
	if c == nil || c.zalo == nil {
		return zalo.Status{}
	}
	return c.zalo.Status()
}
