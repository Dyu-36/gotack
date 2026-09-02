package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Dyu-36/gotack/internal/openaioauth"
)

func TestProviderUsageFromChatGPTPreservesProviderWindows(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	usage := providerUsageFromChatGPT(chatGPTUsagePayload{
		PlanType: "plus",
		RateLimit: &chatGPTRateLimitDetails{
			Allowed: true,
			PrimaryWindow: &chatGPTRateLimitWindow{
				UsedPercent:        36,
				LimitWindowSeconds: 18_000,
				ResetAt:            1_700_003_600,
			},
			SecondaryWindow: &chatGPTRateLimitWindow{
				UsedPercent:        72,
				LimitWindowSeconds: 604_800,
				ResetAt:            1_700_086_400,
			},
		},
		AdditionalRateLimits: []chatGPTAdditionalRateLimit{{
			LimitName: "Codex Spark",
			RateLimit: &chatGPTRateLimitDetails{
				Allowed:      false,
				LimitReached: true,
				PrimaryWindow: &chatGPTRateLimitWindow{
					UsedPercent:        101,
					LimitWindowSeconds: 86_400,
					ResetAfterSeconds:  900,
				},
			},
		}},
	}, now)

	if !usage.Available {
		t.Fatal("usage should be available")
	}
	if usage.Plan != "plus" {
		t.Fatalf("plan = %q, want plus", usage.Plan)
	}
	if !usage.LimitReached {
		t.Fatal("limit_reached = false, want true")
	}
	if len(usage.Windows) != 3 {
		t.Fatalf("windows = %d, want 3", len(usage.Windows))
	}
	if got := usage.Windows[0]; got.WindowSeconds != 18_000 || got.RemainingPercent != 64 {
		t.Fatalf("five-hour window = %#v", got)
	}
	if got := usage.Windows[1]; got.Name != "Codex Spark" || got.RemainingPercent != 0 || got.ResetsAt != now.Add(15*time.Minute).UnixMilli() {
		t.Fatalf("additional window = %#v", got)
	}
	if got := usage.Windows[2]; got.WindowSeconds != 604_800 || got.RemainingPercent != 28 {
		t.Fatalf("weekly window = %#v", got)
	}
}

func TestFetchChatGPTUsageSendsAccountScopedHeaders(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer access-token" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("ChatGPT-Account-Id"); got != "account-123" {
			t.Errorf("ChatGPT-Account-Id = %q", got)
		}
		if got := r.Header.Get("X-OpenAI-Fedramp"); got != "true" {
			t.Errorf("X-OpenAI-Fedramp = %q", got)
		}
		if got := r.Header.Get("User-Agent"); got != "gotack" {
			t.Errorf("User-Agent = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"plan_type":"pro","rate_limit":{"allowed":true,"limit_reached":false,"primary_window":{"used_percent":25,"limit_window_seconds":18000,"reset_after_seconds":60,"reset_at":0}}}`))
	}))
	defer server.Close()

	now := time.Unix(1_700_000_000, 0)
	usage, err := fetchChatGPTUsage(context.Background(), server.Client(), server.URL, openaioauth.Token{
		AccessToken:    "access-token",
		AccountID:      "account-123",
		AccountFedRAMP: true,
	}, now)
	if err != nil {
		t.Fatalf("fetchChatGPTUsage() error = %v", err)
	}
	if len(usage.Windows) != 1 || usage.Windows[0].RemainingPercent != 75 {
		t.Fatalf("usage = %#v", usage)
	}
	if usage.Windows[0].ResetsAt != now.Add(time.Minute).UnixMilli() {
		t.Fatalf("resets_at = %d", usage.Windows[0].ResetsAt)
	}
}

func TestGetProviderUsageReturnsUnavailableForUnsupportedProvider(t *testing.T) {
	t.Parallel()

	usage, err := NewApp().GetProviderUsage("anthropic")
	if err != nil {
		t.Fatalf("GetProviderUsage() error = %v", err)
	}
	if usage.Available {
		t.Fatal("unsupported provider must not report available usage")
	}
	if usage.ProviderID != "anthropic" || usage.UnavailableReason == "" {
		t.Fatalf("usage = %#v", usage)
	}
}
