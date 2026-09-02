package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/zalo"
)

type ZaloConfigInfo struct {
	Enabled     bool     `json:"enabled"`
	PairedChats []string `json:"paired_chats"`
	PairingCode string   `json:"pairing_code"`
	HasToken    bool     `json:"has_token"`
	BotName     string   `json:"bot_name,omitempty"`
	TokenSuffix string   `json:"token_suffix,omitempty"`
	Running     bool     `json:"running"`
}

type ZaloConfigUpdate struct {
	Enabled bool   `json:"enabled"`
	Token   string `json:"token,omitempty"`
}

type ZaloFileRequest struct {
	Path   string `json:"path"`
	ChatID string `json:"chat_id,omitempty"`
}

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

func (a *App) GetZaloConfig() ZaloConfigInfo {
	return a.snapshotZaloConfig()
}

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

func (a *App) TestZaloConnection() (ZaloManagerStatus, error) {
	if a.zalo == nil {
		return ZaloManagerStatus{}, errors.New("zalo manager not initialised")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return a.zalo.TestConnection(ctx)
}

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
		//lint:ignore SA1019 legacy Zalo config cleanup remains supported until Gotack v1.0.
		a.cfg.Zalo.Token = ""
		//lint:ignore SA1019 legacy Zalo config cleanup remains supported until Gotack v1.0.
		a.cfg.Zalo.AllowedChats = nil
		_ = appconfig.Save(a.cfg)
	}
	return status, nil
}

func (a *App) RegenerateZaloPairingCode() (ZaloManagerStatus, error) {
	if a.zalo == nil {
		return ZaloManagerStatus{}, errors.New("zalo manager not initialised")
	}
	return a.zalo.RegeneratePairingCode()
}

func (a *App) UnpairZaloChat(chatID string) (ZaloManagerStatus, error) {
	if a.zalo == nil {
		return ZaloManagerStatus{}, errors.New("zalo manager not initialised")
	}
	return a.zalo.Unpair(chatID)
}

func (a *App) ZaloStatus() ZaloManagerStatus {
	if a.zalo == nil {
		return ZaloManagerStatus{}
	}
	return a.zalo.Status()
}

func (a *App) SendZaloFile(req ZaloFileRequest) (string, error) {
	if a.zalo == nil {
		return "", errors.New("zalo manager not initialised")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	return a.zalo.SendFile(ctx, req.Path, strings.TrimSpace(req.ChatID))
}
