package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/crushapi"
)

// bind_config.go -- role: Wails-bound API for user settings, theme, provider
// and model configuration.

// SettingsInfo is the settings payload exchanged with the UI. Every field must
// be genuinely readable and writable (AGENTS.md hard rule 8).
type SettingsInfo struct {
	Theme              string `json:"theme"`
	Provider           string `json:"provider"`
	CredentialProvider string `json:"credential_provider,omitempty"`
	ProviderOnly       bool   `json:"provider_only,omitempty"`
	Model              string `json:"model"`
	Thinking           string `json:"thinking"`
	// APIKey is write-only from the UI. GetSettings always returns an empty
	// value so a credential is never round-tripped through Wails state.
	APIKey    string `json:"api_key"`
	CustomURL string `json:"custom_url"`
}

// GetSettings returns non-secret Gotack preferences. Provider credentials are
// owned by Crush and deliberately never returned to the webview.
func (a *App) GetSettings() SettingsInfo {
	if a.cfg == nil {
		return SettingsInfo{Theme: "system"}
	}
	return SettingsInfo{
		Theme:     a.cfg.Theme,
		Provider:  a.cfg.Provider,
		Model:     a.cfg.Model,
		Thinking:  a.cfg.Thinking,
		APIKey:    "",
		CustomURL: a.cfg.CustomURL,
	}
}

var simpleEnvCredentialRef = regexp.MustCompile(`^\$(?:\{([A-Za-z_][A-Za-z0-9_]*)\}|([A-Za-z_][A-Za-z0-9_]*))$`)

// resolvedProviderCredential turns Crush's stored credential representation
// into a usable credential signal. Crush keeps known-provider defaults such as
// "$MINIMAX_API_KEY" in config even when the environment variable is absent;
// those templates must not make a provider look configured in Gotack.
func resolvedProviderCredential(pc crushapi.ProviderConfig) (kind, value string, ok bool) {
	oauth := strings.TrimSpace(string(pc.OAuth))
	if oauth != "" && oauth != "null" && oauth != "{}" {
		return "oauth", "", true
	}

	key := strings.TrimSpace(pc.APIKey)
	if key == "" {
		return "", "", false
	}
	if match := simpleEnvCredentialRef.FindStringSubmatch(key); match != nil {
		name := match[1]
		if name == "" {
			name = match[2]
		}
		resolved, exists := os.LookupEnv(name)
		if !exists || strings.TrimSpace(resolved) == "" {
			return "", "", false
		}
		return "api_key", resolved, true
	}
	// Other leading-$ forms are shell templates supported by Crush. Gotack does
	// not execute arbitrary command substitution merely to populate settings UI;
	// treat them as unverified instead of exposing a false-positive provider.
	if strings.HasPrefix(key, "$") {
		return "", "", false
	}
	return "api_key", key, true
}

// configWorkspaceID returns a workspace ID usable for provider and config
// reads. When the user has no workspace open it falls back to a private
// catalog workspace under Gotack's config directory, without changing what the
// user currently has open.
func (a *App) configWorkspaceID(ctx context.Context, svc *bridgeServices) (string, error) {
	if desc, ok := svc.ws.Current(); ok && desc.WorkspaceID != "" {
		return desc.WorkspaceID, nil
	}
	catalogPath := filepath.Join(appconfig.Dir(), "catalog-workspace")
	if err := os.MkdirAll(catalogPath, 0o755); err != nil {
		return "", fmt.Errorf("create catalog workspace directory: %w", err)
	}
	ws, err := svc.api.CreateWorkspace(ctx, catalogPath, false)
	if err != nil {
		return "", fmt.Errorf("create catalog workspace: %w", err)
	}
	return ws.ID, nil
}

// ListProviders returns the live provider and model catalog from Crush. When
// no user workspace is open, it uses a private workspace under Gotack's config
// directory without changing the user's current workspace.
func (a *App) ListProviders() ([]crushapi.Provider, error) {
	svc, err := a.services()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 90*time.Second)
	defer cancel()

	workspaceID, err := a.configWorkspaceID(ctx, svc)
	if err != nil {
		return nil, err
	}
	providers, err := svc.api.ListProviders(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	providers, localOverlays := mergeLocalProviderOverlays(providers)
	cfg, err := svc.api.GetWorkspaceConfig(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("get resolved Crush config: %w", err)
	}
	for i := range providers {
		pc, exists := cfg.Providers[providers[i].ID]
		if !exists || pc.Disable {
			continue
		}
		if localOverlays[providers[i].ID] {
			if pc.Name != "" {
				providers[i].Name = pc.Name
			}
			if pc.Type != "" {
				providers[i].Type = pc.Type
			}
			if pc.BaseURL != "" {
				providers[i].APIEndpoint = pc.BaseURL
			}
			if len(pc.Models) > 0 {
				providers[i].Models = mergeProviderModels(pc.Models, providers[i].Models)
			}
		}
		kind, _, usable := resolvedProviderCredential(pc)
		if !usable {
			continue
		}
		providers[i].Configured = true
		providers[i].CredentialKind = kind
	}
	if a.cfg != nil && a.cfg.ModelCapabilities != nil {
		for i := range providers {
			for j := range providers[i].Models {
				m := &providers[i].Models[j]
				if override, ok := a.cfg.ModelCapabilities[m.ID]; ok {
					if override.SupportsVision != nil && !*override.SupportsVision {
						// A desktop-only override may safely disable attachments,
						// but cannot enable a capability Crush will reject or strip.
						m.SupportsVision = false
					}
					if override.CanReason != nil {
						m.CanReason = *override.CanReason
					}
				}
			}
		}
	}
	return providers, nil
}

// RevealProviderAPIKey returns a configured provider API key only after an
// explicit UI request (the eye button). It is never included in GetSettings or
// ListProviders, keeping secrets out of normal webview state.
func (a *App) RevealProviderAPIKey(providerID string) (string, error) {
	svc, err := a.services()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()

	workspaceID, err := a.configWorkspaceID(ctx, svc)
	if err != nil {
		return "", err
	}
	cfg, err := svc.api.GetWorkspaceConfig(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	pc, exists := cfg.Providers[strings.TrimSpace(providerID)]
	if !exists || pc.Disable {
		return "", fmt.Errorf("provider %q is not configured", providerID)
	}
	kind, key, usable := resolvedProviderCredential(pc)
	if !usable || kind != "api_key" {
		return "", fmt.Errorf("provider %q does not have a revealable API key", providerID)
	}
	return key, nil
}

// DeleteProvider removes stored credentials for a provider and disables it in
// Crush's effective config. Disabling is necessary for providers whose
// credential comes from an environment variable: deleting only api_key would
// make Crush immediately pick the environment-backed default again.
func (a *App) DeleteProvider(providerID string) error {
	providerID = strings.TrimSpace(providerID)
	if !safeProviderID.MatchString(providerID) {
		return fmt.Errorf("invalid provider id %q", providerID)
	}
	svc, err := a.services()
	if err != nil {
		return err
	}
	desc, ok := svc.ws.Current()
	if !ok || desc.WorkspaceID == "" {
		return fmt.Errorf("no workspace is open")
	}

	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()
	ws, scope := desc.WorkspaceID, crushapi.ConfigScopeGlobal
	base := "providers." + providerID
	engineConfig, err := svc.api.GetWorkspaceConfig(ctx, ws)
	if err != nil {
		return fmt.Errorf("read provider state before deletion: %w", err)
	}
	clearSelection := a.cfg != nil && strings.TrimSpace(a.cfg.Provider) == providerID
	clearModels := clearSelection || preferredModelsUseProvider(engineConfig.Models, providerID)
	if clearSelection {
		next := *a.cfg
		next.Provider = ""
		next.Model = ""
		next.CustomURL = ""
		if err := appconfig.Save(&next); err != nil {
			return fmt.Errorf("save cleared provider selection: %w", err)
		}
		a.cfg = &next
	}

	// Once disabled, every remaining cleanup step is idempotent and safe to
	// retry without leaving an enabled provider with missing credentials.
	if err := svc.api.SetConfigField(ctx, ws, scope, base+".disable", true); err != nil {
		return fmt.Errorf("disable provider: %w", err)
	}
	if clearModels {
		if err := svc.api.RemovePreferredModelPair(ctx, ws, scope); err != nil {
			return fmt.Errorf("clear provider model selection: %w", err)
		}
	}
	if err := svc.api.RemoveConfigField(ctx, ws, scope, base+".api_key"); err != nil {
		return fmt.Errorf("remove provider API key: %w", err)
	}
	if err := svc.api.RemoveConfigField(ctx, ws, scope, base+".oauth"); err != nil {
		return fmt.Errorf("remove provider OAuth credential: %w", err)
	}
	return nil
}

func preferredModelsUseProvider(models map[string]crushapi.SelectedModel, providerID string) bool {
	for _, modelType := range []string{"large", "small"} {
		if strings.TrimSpace(models[modelType].Provider) == providerID {
			return true
		}
	}
	return false
}

// SaveSettings persists non-secret UI preferences and applies agent-affecting
// settings through Crush's REST API. API keys are sent directly to Crush's
// provider-key endpoint and are never written to Gotack's config.json.
func (a *App) SaveSettings(s SettingsInfo) error {
	apiKey := strings.TrimSpace(s.APIKey)
	// The host repoints a selection stranded by the provider split while it
	// applies it, so persist what the engine received instead of what the UI
	// sent: the two differ exactly when the stale selection was corrected.
	effective, err := a.applyEffectiveCrushSettings(s, apiKey)
	if err != nil {
		return err
	}

	current := a.cfg
	if current == nil {
		current = appconfig.Defaults()
	}
	next := *current
	if effective.Theme != "" {
		next.Theme = effective.Theme
	}
	next.Provider = strings.TrimSpace(effective.Provider)
	next.Model = strings.TrimSpace(effective.Model)
	// Gotack exposes one model selector. applyCrushSettings pins both
	// models.large and models.small to it, so there is nothing extra to persist.
	next.Thinking = strings.TrimSpace(effective.Thinking)
	next.APIKey = "" // scrub any credential persisted by older builds
	credentialProvider := strings.TrimSpace(effective.CredentialProvider)
	if credentialProvider == "" || credentialProvider == next.Provider {
		next.CustomURL = strings.TrimSpace(effective.CustomURL)
	}
	if err := appconfig.Save(&next); err != nil {
		return err
	}
	a.cfg = &next
	return nil
}
