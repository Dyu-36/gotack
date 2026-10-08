package config

import (
	"fmt"
	"maps"

	"charm.land/catwalk/pkg/catwalk"
	openaioauth "github.com/Dyu-36/gotack/internal/agentcore/oauth/openai"
)

// ModelRoute describes a model's wire protocol and optional endpoint defaults.
// Headers are literal defaults; user-configured extra_headers take precedence.
type ModelRoute struct {
	API     string            `json:"api"`
	BaseURL string            `json:"base_url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// ForModel applies a catalog route without mutating shared provider config.
// Account-scoped subscription routing always remains authoritative.
func (p ProviderConfig) ForModel(modelID string) (ProviderConfig, error) {
	if openaioauth.HasSubscriptionCredential(p.ID, p.OAuthToken) {
		return p, nil
	}
	route, exists := p.ModelRoutes[modelID]
	if !exists {
		return p, nil
	}
	switch route.API {
	case "openai-responses":
		p.Type = catwalk.TypeOpenAI
	case "openai-completions":
		if p.Type != catwalk.TypeOpenRouter && p.Type != catwalk.TypeVercel {
			p.Type = catwalk.TypeOpenAICompat
		}
	case "anthropic-messages":
		p.Type = catwalk.TypeAnthropic
	case "google-generative-ai":
		p.Type = catwalk.TypeGoogle
	case "google-vertex":
		p.Type = catwalk.TypeVertexAI
	case "bedrock-converse-stream":
		p.Type = catwalk.TypeBedrock
	case "azure-openai-responses":
		p.Type = catwalk.TypeAzure
	default:
		return p, fmt.Errorf("unsupported catalog api %q for provider %q model %q", route.API, p.ID, modelID)
	}
	if route.BaseURL != "" {
		p.BaseURL = route.BaseURL
	}
	headers := maps.Clone(route.Headers)
	if headers == nil {
		headers = make(map[string]string)
	}
	maps.Copy(headers, p.ExtraHeaders)
	p.ExtraHeaders = headers
	return p, nil
}

// UsesResponsesAPI honors the model route before the legacy name heuristic.
func (p ProviderConfig) UsesResponsesAPI(modelID string, legacy bool) bool {
	if openaioauth.HasSubscriptionCredential(p.ID, p.OAuthToken) {
		return legacy
	}
	if route, exists := p.ModelRoutes[modelID]; exists {
		return route.API == "openai-responses" || route.API == "azure-openai-responses"
	}
	return legacy
}
