package provider

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/Dyu-36/gotack/internal/engineapi"
)

var safeID = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type Settings struct {
	Theme              string
	Provider           string
	CredentialProvider string
	ProviderOnly       bool
	Model              string
	Thinking           string
	CustomURL          string
}

func ValidID(providerID string) bool {
	return safeID.MatchString(providerID)
}

func Reasoning(value string) (effort string, think bool) {
	switch normalized := strings.ToLower(strings.TrimSpace(value)); normalized {
	case "none", "off":
		return "none", false
	case "minimal", "low", "medium", "high", "xhigh", "max":
		return normalized, true
	default:
		return "", false
	}
}

func ProviderUsesOAuth(ctx context.Context, api *engineapi.Client, workspaceID, providerID string) (bool, error) {
	if providerID == CodexID {
		return true, nil
	}
	cfg, err := api.GetWorkspaceConfig(ctx, workspaceID)
	if err != nil {
		return false, fmt.Errorf("read the provider credential kind: %w", err)
	}
	configured, ok := cfg.Providers[providerID]
	return ok && OAuthCredentialPresent(configured), nil
}

func Apply(ctx context.Context, api *engineapi.Client, workspaceID string, settings Settings, apiKey string) error {
	providerID := strings.TrimSpace(settings.Provider)
	credentialProvider := strings.TrimSpace(settings.CredentialProvider)
	if credentialProvider == "" {
		credentialProvider = providerID
	}
	modelID := strings.TrimSpace(settings.Model)
	endpoint := strings.TrimSpace(settings.CustomURL)

	if apiKey != "" && credentialProvider == "" {
		return errors.New("provider is required before storing an API key")
	}
	if apiKey != "" && credentialProvider == CodexID {
		return errors.New("codex signs in with ChatGPT, not an API key; use the openai provider for an API key")
	}
	if (settings.ProviderOnly || endpoint != "") && !ValidID(credentialProvider) {
		return fmt.Errorf("provider id %q cannot be used in an engine config path", credentialProvider)
	}
	if endpoint != "" {
		oauthBacked, err := ProviderUsesOAuth(ctx, api, workspaceID, credentialProvider)
		if err != nil {
			return err
		}
		if oauthBacked {
			return fmt.Errorf("provider %q signs in with OAuth and does not accept a custom endpoint", credentialProvider)
		}
	}

	scope := engineapi.ConfigScopeGlobal
	managedLocalProvider := false
	var err error
	if credentialProvider != "" {
		managedLocalProvider, err = PrepareLocal(ctx, api, workspaceID, scope, credentialProvider)
		if err != nil {
			return err
		}
	}
	if credentialProvider != "" && settings.ProviderOnly {
		if err := api.SetConfigField(ctx, workspaceID, scope, "providers."+credentialProvider+".disable", false); err != nil {
			return fmt.Errorf("enable provider: %w", err)
		}
	}
	if apiKey != "" {
		if err := api.SetProviderAPIKey(ctx, workspaceID, scope, credentialProvider, apiKey); err != nil {
			return fmt.Errorf("apply engine provider credential: %w", err)
		}
	}
	if endpoint != "" {
		if err := api.SetConfigField(ctx, workspaceID, scope, "providers."+credentialProvider+".base_url", endpoint); err != nil {
			return fmt.Errorf("apply engine provider endpoint: %w", err)
		}
	}
	if managedLocalProvider {
		if err := FinalizeLocal(ctx, api, workspaceID, scope, credentialProvider); err != nil {
			return err
		}
	}
	if !settings.ProviderOnly && providerID != "" && modelID != "" {
		effort, think := Reasoning(settings.Thinking)
		selected := engineapi.SelectedModel{Provider: providerID, Model: modelID, ReasoningEffort: effort, Think: think}
		if err := api.SetPreferredModelPair(ctx, workspaceID, scope, selected); err != nil {
			return fmt.Errorf("apply engine model selection: %w", err)
		}
	}
	if providerID != "" && modelID != "" {
		if err := api.EnsureAgent(ctx, workspaceID, true); err != nil {
			return fmt.Errorf("initialize engine agent: %w", err)
		}
	}
	return nil
}

func PreferredModelsUseProvider(models map[string]engineapi.SelectedModel, providerID string) bool {
	for _, modelType := range []string{"large", "small"} {
		if strings.TrimSpace(models[modelType].Provider) == providerID {
			return true
		}
	}
	return false
}

func DeleteEngineConfig(ctx context.Context, api *engineapi.Client, workspaceID, providerID string, clearModels bool) error {
	base := "providers." + providerID
	scope := engineapi.ConfigScopeGlobal
	if err := api.SetConfigField(ctx, workspaceID, scope, base+".disable", true); err != nil {
		return fmt.Errorf("disable provider: %w", err)
	}
	if clearModels {
		if err := api.RemovePreferredModelPair(ctx, workspaceID, scope); err != nil {
			return fmt.Errorf("clear provider model selection: %w", err)
		}
	}
	if err := api.RemoveConfigField(ctx, workspaceID, scope, base+".api_key"); err != nil {
		return fmt.Errorf("remove provider API key: %w", err)
	}
	if err := api.RemoveConfigField(ctx, workspaceID, scope, base+".oauth"); err != nil {
		return fmt.Errorf("remove provider OAuth credential: %w", err)
	}
	return nil
}
