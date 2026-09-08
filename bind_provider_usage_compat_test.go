package main

import (
	"context"
	"net/http"
	"time"

	providerdomain "github.com/Dyu-36/gotack/internal/provider"
)

type chatGPTUsagePayload = providerdomain.ChatGPTUsagePayload
type chatGPTAdditionalRateLimit = providerdomain.ChatGPTAdditionalRateLimit
type chatGPTRateLimitDetails = providerdomain.ChatGPTRateLimitDetails
type chatGPTRateLimitWindow = providerdomain.ChatGPTRateLimitWindow

func fetchChatGPTUsage(ctx context.Context, client *http.Client, endpoint string, token providerdomain.OpenAIOAuthToken, now time.Time) (ProviderUsageInfo, error) {
	usage, err := providerdomain.FetchChatGPTUsage(ctx, client, endpoint, token, now)
	return providerUsageInfoFromDomain(usage), err
}

func providerUsageFromChatGPT(payload chatGPTUsagePayload, now time.Time) ProviderUsageInfo {
	return providerUsageInfoFromDomain(providerdomain.UsageFromChatGPT(payload, now))
}
