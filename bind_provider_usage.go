package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/openaioauth"
)

const chatGPTUsageEndpoint = "https://chatgpt.com/backend-api/wham/usage"

var providerUsageHTTPClient = &http.Client{
	Timeout: 12 * time.Second,
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		// Never forward a bearer token to a redirect destination.
		return http.ErrUseLastResponse
	},
}

// ProviderUsageWindow is one provider-defined quota window. Providers may
// expose several independent windows (for example five-hour and weekly caps).
type ProviderUsageWindow struct {
	ID               string  `json:"id"`
	Name             string  `json:"name,omitempty"`
	UsedPercent      float64 `json:"used_percent"`
	RemainingPercent float64 `json:"remaining_percent"`
	WindowSeconds    int64   `json:"window_seconds,omitempty"`
	ResetsAt         int64   `json:"resets_at,omitempty"`
}

// ProviderUsageInfo is the provider-neutral quota payload consumed by the UI.
// Remaining values are percentages because subscription providers do not
// expose a reliable absolute token balance through their account APIs.
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

type chatGPTUsagePayload struct {
	PlanType             string                       `json:"plan_type"`
	RateLimit            *chatGPTRateLimitDetails     `json:"rate_limit"`
	AdditionalRateLimits []chatGPTAdditionalRateLimit `json:"additional_rate_limits"`
}

type chatGPTAdditionalRateLimit struct {
	LimitName      string                   `json:"limit_name"`
	MeteredFeature string                   `json:"metered_feature"`
	RateLimit      *chatGPTRateLimitDetails `json:"rate_limit"`
}

type chatGPTRateLimitDetails struct {
	Allowed         bool                    `json:"allowed"`
	LimitReached    bool                    `json:"limit_reached"`
	PrimaryWindow   *chatGPTRateLimitWindow `json:"primary_window"`
	SecondaryWindow *chatGPTRateLimitWindow `json:"secondary_window"`
}

type chatGPTRateLimitWindow struct {
	UsedPercent        float64 `json:"used_percent"`
	LimitWindowSeconds int64   `json:"limit_window_seconds"`
	ResetAfterSeconds  int64   `json:"reset_after_seconds"`
	ResetAt            int64   `json:"reset_at"`
}

// GetProviderUsage returns the selected provider's account quota windows. The
// contract is provider-neutral; adapters are added only when the provider has a
// documented account-usage signal. Codex is currently the only configured
// subscription provider with such a signal in Gotack.
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
		return unavailableProviderUsage(
			providerID,
			providerID,
			"Provider này chưa công bố hạn mức theo phiên qua API.",
			now,
		), nil
	}
	return a.getChatGPTProviderUsage(now)
}

func (a *App) getChatGPTProviderUsage(now time.Time) (ProviderUsageInfo, error) {
	unavailable := unavailableProviderUsage(
		codexProviderID,
		"ChatGPT (Codex)",
		"Chưa đăng nhập ChatGPT.",
		now,
	)

	svc, err := a.services()
	if err != nil {
		return unavailable, err
	}
	baseCtx := a.ctx
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	ctx, cancel := context.WithTimeout(baseCtx, 15*time.Second)
	defer cancel()

	workspaceID, err := a.configWorkspaceID(ctx, svc)
	if err != nil {
		return unavailable, err
	}

	// Keep pre-provider-split installs working even when this is the first OAuth
	// surface the user opens after upgrading.
	if moved, moveErr := migrateChatGPTOAuthToCodex(ctx, svc.api, workspaceID); moveErr != nil {
		a.warnCodexMigration("could not move the ChatGPT credential before loading usage", "err", moveErr)
	} else if moved {
		a.repointSavedModelAtCodex(svc, workspaceID)
	}

	cfg, err := svc.api.GetWorkspaceConfig(ctx, workspaceID)
	if err != nil {
		return unavailable, fmt.Errorf("get Crush config for provider usage: %w", err)
	}
	tok, ok := configuredChatGPTToken(cfg.Providers[codexProviderID])
	if !ok {
		return unavailable, nil
	}

	if tok.ExpiresAt > 0 && now.Add(time.Minute).Unix() >= tok.ExpiresAt {
		if tok.RefreshToken == "" {
			unavailable.UnavailableReason = "Phiên ChatGPT đã hết hạn; hãy đăng nhập lại."
			return unavailable, nil
		}
		if err := svc.api.RefreshProviderOAuthToken(ctx, workspaceID, crushapi.ConfigScopeGlobal, codexProviderID); err != nil {
			if a.log != nil {
				a.log.Warn("refresh ChatGPT OAuth before loading provider usage failed", "err", err)
			}
			unavailable.UnavailableReason = "Không thể làm mới phiên ChatGPT; hãy đăng nhập lại."
			return unavailable, nil
		}
		cfg, err = svc.api.GetWorkspaceConfig(ctx, workspaceID)
		if err != nil {
			return unavailable, fmt.Errorf("get refreshed Crush config for provider usage: %w", err)
		}
		tok, ok = configuredChatGPTToken(cfg.Providers[codexProviderID])
		if !ok {
			return unavailable, nil
		}
	}

	if tok.AccountID == "" && tok.IDToken != "" {
		metadata := openaioauth.ParseIDTokenMetadata(tok.IDToken)
		tok.AccountID = metadata.AccountID
		tok.AccountFedRAMP = metadata.AccountFedRAMP
	}
	if tok.AccountID == "" {
		unavailable.UnavailableReason = "Thông tin tài khoản ChatGPT không đầy đủ; hãy đăng nhập lại."
		return unavailable, nil
	}

	usage, err := fetchChatGPTUsage(ctx, providerUsageHTTPClient, chatGPTUsageEndpoint, tok, now)
	if err != nil {
		return unavailable, err
	}
	if usage.Plan == "" {
		usage.Plan = tok.UserPlan()
	}
	return usage, nil
}

func configuredChatGPTToken(provider crushapi.ProviderConfig) (openaioauth.Token, bool) {
	if provider.Disable {
		return openaioauth.Token{}, false
	}
	raw := strings.TrimSpace(string(provider.OAuth))
	if raw == "" || raw == "null" || raw == "{}" {
		return openaioauth.Token{}, false
	}
	var tok openaioauth.Token
	if err := json.Unmarshal(provider.OAuth, &tok); err != nil || strings.TrimSpace(tok.AccessToken) == "" {
		return openaioauth.Token{}, false
	}
	return tok, true
}

func fetchChatGPTUsage(
	ctx context.Context,
	client *http.Client,
	endpoint string,
	tok openaioauth.Token,
	now time.Time,
) (ProviderUsageInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ProviderUsageInfo{}, fmt.Errorf("create ChatGPT usage request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Set("ChatGPT-Account-Id", tok.AccountID)
	req.Header.Set("User-Agent", "gotack")
	if tok.AccountFedRAMP {
		req.Header.Set("X-OpenAI-Fedramp", "true")
	}

	resp, err := client.Do(req)
	if err != nil {
		return ProviderUsageInfo{}, fmt.Errorf("load ChatGPT provider usage: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return ProviderUsageInfo{}, fmt.Errorf("ChatGPT rejected the usage request; sign in again")
		}
		return ProviderUsageInfo{}, fmt.Errorf("load ChatGPT provider usage: HTTP %s", resp.Status)
	}

	var payload chatGPTUsagePayload
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if err := decoder.Decode(&payload); err != nil {
		return ProviderUsageInfo{}, fmt.Errorf("decode ChatGPT provider usage: %w", err)
	}
	return providerUsageFromChatGPT(payload, now), nil
}

func providerUsageFromChatGPT(payload chatGPTUsagePayload, now time.Time) ProviderUsageInfo {
	usage := ProviderUsageInfo{
		ProviderID:   codexProviderID,
		ProviderName: "ChatGPT (Codex)",
		Plan:         payload.PlanType,
		Windows:      []ProviderUsageWindow{},
		UpdatedAt:    now.UnixMilli(),
	}

	appendRateLimitWindows := func(name string, details *chatGPTRateLimitDetails) {
		if details == nil {
			return
		}
		usage.LimitReached = usage.LimitReached || details.LimitReached || !details.Allowed
		for index, window := range []*chatGPTRateLimitWindow{details.PrimaryWindow, details.SecondaryWindow} {
			if window == nil || window.LimitWindowSeconds <= 0 {
				continue
			}
			used := clampUsagePercent(window.UsedPercent)
			resetsAt := window.ResetAt * 1000
			if resetsAt <= 0 && window.ResetAfterSeconds > 0 {
				resetsAt = now.Add(time.Duration(window.ResetAfterSeconds) * time.Second).UnixMilli()
			}
			usage.Windows = append(usage.Windows, ProviderUsageWindow{
				ID:               fmt.Sprintf("%s:%d:%d", usageWindowID(name, index), window.LimitWindowSeconds, resetsAt),
				Name:             name,
				UsedPercent:      used,
				RemainingPercent: 100 - used,
				WindowSeconds:    window.LimitWindowSeconds,
				ResetsAt:         resetsAt,
			})
		}
	}

	appendRateLimitWindows("", payload.RateLimit)
	for _, additional := range payload.AdditionalRateLimits {
		name := strings.TrimSpace(additional.LimitName)
		if name == "" {
			name = strings.TrimSpace(additional.MeteredFeature)
		}
		appendRateLimitWindows(name, additional.RateLimit)
	}

	sort.SliceStable(usage.Windows, func(i, j int) bool {
		if usage.Windows[i].WindowSeconds == usage.Windows[j].WindowSeconds {
			return usage.Windows[i].Name < usage.Windows[j].Name
		}
		return usage.Windows[i].WindowSeconds < usage.Windows[j].WindowSeconds
	})
	usage.Available = len(usage.Windows) > 0
	if !usage.Available {
		usage.UnavailableReason = "ChatGPT không trả về cửa sổ hạn mức cho tài khoản này."
	}
	return usage
}

func unavailableProviderUsage(providerID, providerName, reason string, now time.Time) ProviderUsageInfo {
	return ProviderUsageInfo{
		ProviderID:        providerID,
		ProviderName:      providerName,
		Windows:           []ProviderUsageWindow{},
		UpdatedAt:         now.UnixMilli(),
		UnavailableReason: reason,
	}
}

func clampUsagePercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func usageWindowID(name string, index int) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		default:
			return '-'
		}
	}, name)
	name = strings.Trim(name, "-")
	if name == "" {
		name = "account"
	}
	return fmt.Sprintf("%s-%d", name, index)
}
