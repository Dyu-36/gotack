package main

import (
	"context"
	"net/http"
	"strings"
	"time"

	providerdomain "github.com/Dyu-36/gotack/internal/provider"
)

const chatGPTUsageEndpoint = "https://chatgpt.com/backend-api/wham/usage"

var providerUsageHTTPClient = &http.Client{
	Timeout: 12 * time.Second,
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

type ProviderUsageWindow struct {
	ID               string  `json:"id"`
	Name             string  `json:"name,omitempty"`
	UsedPercent      float64 `json:"used_percent"`
	RemainingPercent float64 `json:"remaining_percent"`
	WindowSeconds    int64   `json:"window_seconds,omitempty"`
	ResetsAt         int64   `json:"resets_at,omitempty"`
}

type ProviderUsageInfo struct {
	ProviderID        string                `json:"provider_id"`
	ProviderName      string                `json:"provider_name"`
	Available         bool                  `json:"available"`
	Plan              string                `json:"plan,omitempty"`
	LimitReached      bool                  `json:"limit_reached"`
	Windows           []ProviderUsageWindow `json:"windows"`
	UpdatedAt         int64                 `json:"updated_at"`
	UnavailableReason string                `json:"unavailable_reason,omitempty"`
}

type chatGPTUsagePayload = providerdomain.ChatGPTUsagePayload
type chatGPTAdditionalRateLimit = providerdomain.ChatGPTAdditionalRateLimit
type chatGPTRateLimitDetails = providerdomain.ChatGPTRateLimitDetails
type chatGPTRateLimitWindow = providerdomain.ChatGPTRateLimitWindow

func (a *App) GetProviderUsage(providerID string) (ProviderUsageInfo, error) {
	providerID = strings.TrimSpace(providerID)
	if providerID == "" && a.cfg != nil {
		providerID = strings.TrimSpace(a.cfg.Provider)
	}
	now := time.Now()
	if providerID == "" {
		return unavailableProviderUsage("", "", "Chưa chọn provider.", now), nil
	}
	if providerID != codexProviderID {
		return unavailableProviderUsage(providerID, providerID, "Provider này chưa công bố hạn mức theo phiên qua API.", now), nil
	}
	return a.getChatGPTProviderUsage(now)
}

func (a *App) getChatGPTProviderUsage(now time.Time) (ProviderUsageInfo, error) {
	svc, err := a.services()
	if err != nil {
		return unavailableProviderUsage(codexProviderID, codexProviderName, "Chưa đăng nhập ChatGPT.", now), err
	}
	base := a.ctx
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithTimeout(base, 15*time.Second)
	defer cancel()
	workspaceID, err := a.configWorkspaceID(ctx, svc)
	if err != nil {
		return unavailableProviderUsage(codexProviderID, codexProviderName, "Chưa đăng nhập ChatGPT.", now), err
	}
	if moved, moveErr := providerdomain.MigrateChatGPTOAuthToCodex(ctx, svc.api, workspaceID); moveErr != nil {
		a.warnCodexMigration("could not move the ChatGPT credential before loading usage", "err", moveErr)
	} else if moved {
		a.repointSavedModelAtCodex(svc, workspaceID)
	}
	usage, err := providerdomain.LoadChatGPTUsage(ctx, svc.api, workspaceID, providerUsageHTTPClient, chatGPTUsageEndpoint, now)
	return providerUsageInfoFromDomain(usage), err
}

func fetchChatGPTUsage(ctx context.Context, client *http.Client, endpoint string, token providerdomain.OpenAIOAuthToken, now time.Time) (ProviderUsageInfo, error) {
	usage, err := providerdomain.FetchChatGPTUsage(ctx, client, endpoint, token, now)
	return providerUsageInfoFromDomain(usage), err
}

func providerUsageFromChatGPT(payload chatGPTUsagePayload, now time.Time) ProviderUsageInfo {
	return providerUsageInfoFromDomain(providerdomain.UsageFromChatGPT(payload, now))
}

func unavailableProviderUsage(providerID, providerName, reason string, now time.Time) ProviderUsageInfo {
	return providerUsageInfoFromDomain(providerdomain.UnavailableUsage(providerID, providerName, reason, now))
}

func providerUsageInfoFromDomain(usage providerdomain.UsageInfo) ProviderUsageInfo {
	windows := make([]ProviderUsageWindow, len(usage.Windows))
	for i, window := range usage.Windows {
		windows[i] = ProviderUsageWindow{
			ID:               window.ID,
			Name:             window.Name,
			UsedPercent:      window.UsedPercent,
			RemainingPercent: window.RemainingPercent,
			WindowSeconds:    window.WindowSeconds,
			ResetsAt:         window.ResetsAt,
		}
	}
	return ProviderUsageInfo{
		ProviderID:        usage.ProviderID,
		ProviderName:      usage.ProviderName,
		Available:         usage.Available,
		Plan:              usage.Plan,
		LimitReached:      usage.LimitReached,
		Windows:           windows,
		UpdatedAt:         usage.UpdatedAt,
		UnavailableReason: usage.UnavailableReason,
	}
}
