package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Dyu-36/gotack/internal/engineapi"
	"github.com/Dyu-36/gotack/internal/openaioauth"
)

type UsageWindow struct {
	ID               string  `json:"id"`
	Name             string  `json:"name,omitempty"`
	UsedPercent      float64 `json:"used_percent"`
	RemainingPercent float64 `json:"remaining_percent"`
	WindowSeconds    int64   `json:"window_seconds,omitempty"`
	ResetsAt         int64   `json:"resets_at,omitempty"`
}

type UsageInfo struct {
	ProviderID        string        `json:"provider_id"`
	ProviderName      string        `json:"provider_name"`
	Available         bool          `json:"available"`
	Plan              string        `json:"plan,omitempty"`
	LimitReached      bool          `json:"limit_reached"`
	Windows           []UsageWindow `json:"windows"`
	UpdatedAt         int64         `json:"updated_at"`
	UnavailableReason string        `json:"unavailable_reason,omitempty"`
}

type ChatGPTUsagePayload struct {
	PlanType             string                       `json:"plan_type"`
	RateLimit            *ChatGPTRateLimitDetails     `json:"rate_limit"`
	AdditionalRateLimits []ChatGPTAdditionalRateLimit `json:"additional_rate_limits"`
}

type ChatGPTAdditionalRateLimit struct {
	LimitName      string                   `json:"limit_name"`
	MeteredFeature string                   `json:"metered_feature"`
	RateLimit      *ChatGPTRateLimitDetails `json:"rate_limit"`
}

type ChatGPTRateLimitDetails struct {
	Allowed         bool                    `json:"allowed"`
	LimitReached    bool                    `json:"limit_reached"`
	PrimaryWindow   *ChatGPTRateLimitWindow `json:"primary_window"`
	SecondaryWindow *ChatGPTRateLimitWindow `json:"secondary_window"`
}

type ChatGPTRateLimitWindow struct {
	UsedPercent        float64 `json:"used_percent"`
	LimitWindowSeconds int64   `json:"limit_window_seconds"`
	ResetAfterSeconds  int64   `json:"reset_after_seconds"`
	ResetAt            int64   `json:"reset_at"`
}

func ConfiguredChatGPTToken(config engineapi.ProviderConfig) (openaioauth.Token, bool) {
	if config.Disable {
		return openaioauth.Token{}, false
	}
	raw := strings.TrimSpace(string(config.OAuth))
	if raw == "" || raw == "null" || raw == "{}" {
		return openaioauth.Token{}, false
	}
	var token openaioauth.Token
	if err := json.Unmarshal(config.OAuth, &token); err != nil || strings.TrimSpace(token.AccessToken) == "" {
		return openaioauth.Token{}, false
	}
	return token, true
}

func FetchChatGPTUsage(ctx context.Context, client *http.Client, endpoint string, token openaioauth.Token, now time.Time) (UsageInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return UsageInfo{}, fmt.Errorf("create ChatGPT usage request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("ChatGPT-Account-Id", token.AccountID)
	req.Header.Set("User-Agent", "gotack")
	if token.AccountFedRAMP {
		req.Header.Set("X-OpenAI-Fedramp", "true")
	}

	resp, err := client.Do(req)
	if err != nil {
		return UsageInfo{}, fmt.Errorf("load ChatGPT provider usage: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return UsageInfo{}, fmt.Errorf("ChatGPT rejected the usage request; sign in again")
		}
		return UsageInfo{}, fmt.Errorf("load ChatGPT provider usage: HTTP %s", resp.Status)
	}

	var payload ChatGPTUsagePayload
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if err := decoder.Decode(&payload); err != nil {
		return UsageInfo{}, fmt.Errorf("decode ChatGPT provider usage: %w", err)
	}
	return UsageFromChatGPT(payload, now), nil
}

func UsageFromChatGPT(payload ChatGPTUsagePayload, now time.Time) UsageInfo {
	usage := UsageInfo{
		ProviderID:   CodexID,
		ProviderName: CodexName,
		Plan:         payload.PlanType,
		Windows:      []UsageWindow{},
		UpdatedAt:    now.UnixMilli(),
	}

	appendRateLimitWindows := func(name string, details *ChatGPTRateLimitDetails) {
		if details == nil {
			return
		}
		usage.LimitReached = usage.LimitReached || details.LimitReached || !details.Allowed
		for index, window := range []*ChatGPTRateLimitWindow{details.PrimaryWindow, details.SecondaryWindow} {
			if window == nil || window.LimitWindowSeconds <= 0 {
				continue
			}
			used := ClampUsagePercent(window.UsedPercent)
			resetsAt := window.ResetAt * 1000
			if resetsAt <= 0 && window.ResetAfterSeconds > 0 {
				resetsAt = now.Add(time.Duration(window.ResetAfterSeconds) * time.Second).UnixMilli()
			}
			usage.Windows = append(usage.Windows, UsageWindow{
				ID:               fmt.Sprintf("%s:%d:%d", UsageWindowID(name, index), window.LimitWindowSeconds, resetsAt),
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

func UnavailableUsage(providerID, providerName, reason string, now time.Time) UsageInfo {
	return UsageInfo{
		ProviderID:        providerID,
		ProviderName:      providerName,
		Windows:           []UsageWindow{},
		UpdatedAt:         now.UnixMilli(),
		UnavailableReason: reason,
	}
}

func ClampUsagePercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func UsageWindowID(name string, index int) string {
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

func LoadChatGPTUsage(ctx context.Context, api *engineapi.Client, workspaceID string, client *http.Client, endpoint string, now time.Time) (UsageInfo, error) {
	unavailable := UnavailableUsage(CodexID, CodexName, "Chưa đăng nhập ChatGPT.", now)
	cfg, err := api.GetWorkspaceConfig(ctx, workspaceID)
	if err != nil {
		return unavailable, fmt.Errorf("get engine config for provider usage: %w", err)
	}
	token, ok := ConfiguredChatGPTToken(cfg.Providers[CodexID])
	if !ok {
		return unavailable, nil
	}

	if token.ExpiresAt > 0 && now.Add(time.Minute).Unix() >= token.ExpiresAt {
		if token.RefreshToken == "" {
			unavailable.UnavailableReason = "Phiên ChatGPT đã hết hạn; hãy đăng nhập lại."
			return unavailable, nil
		}
		if err := api.RefreshProviderOAuthToken(ctx, workspaceID, engineapi.ConfigScopeGlobal, CodexID); err != nil {
			unavailable.UnavailableReason = "Không thể làm mới phiên ChatGPT; hãy đăng nhập lại."
			return unavailable, nil
		}
		cfg, err = api.GetWorkspaceConfig(ctx, workspaceID)
		if err != nil {
			return unavailable, fmt.Errorf("get refreshed engine config for provider usage: %w", err)
		}
		token, ok = ConfiguredChatGPTToken(cfg.Providers[CodexID])
		if !ok {
			return unavailable, nil
		}
	}

	if token.AccountID == "" && token.IDToken != "" {
		metadata := openaioauth.ParseIDTokenMetadata(token.IDToken)
		token.AccountID = metadata.AccountID
		token.AccountFedRAMP = metadata.AccountFedRAMP
	}
	if token.AccountID == "" {
		unavailable.UnavailableReason = "Thông tin tài khoản ChatGPT không đầy đủ; hãy đăng nhập lại."
		return unavailable, nil
	}

	usage, err := FetchChatGPTUsage(ctx, client, endpoint, token, now)
	if err != nil {
		return unavailable, err
	}
	if usage.Plan == "" {
		usage.Plan = token.UserPlan()
	}
	return usage, nil
}
