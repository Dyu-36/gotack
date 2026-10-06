package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/Dyu-36/gotack/internal/engineapi"
	"github.com/Dyu-36/gotack/internal/modelcatalog"
)

type piCatalogSource interface {
	Load(context.Context) (modelcatalog.Catalog, error)
}

// PiCatalog projects Pi's model catalog into the engine provider contract and
// keeps the models configured in the engine in step with that catalog.
type PiCatalog struct {
	source       piCatalogSource
	manifestPath string
	mu           sync.Mutex
}

type piCatalogManifest struct {
	Version    int                                             `json:"version"`
	Workspaces map[string]map[string]map[string]piManagedModel `json:"workspaces"`
}

// piModel is the engine's persisted model shape. The manifest stores the last
// value written by PiCatalog so user edits can be distinguished from catalog
// metadata on the next refresh.
type piModel struct {
	ID                     string   `json:"id"`
	Name                   string   `json:"name"`
	ContextWindow          int64    `json:"context_window,omitempty"`
	DefaultMaxTokens       int64    `json:"default_max_tokens,omitempty"`
	CanReason              bool     `json:"can_reason"`
	SupportsAttachments    bool     `json:"supports_attachments"`
	ReasoningLevels        []string `json:"reasoning_levels,omitempty"`
	DefaultReasoningEffort string   `json:"default_reasoning_effort,omitempty"`
	CostPer1MIn            float64  `json:"cost_per_1m_in,omitempty"`
	CostPer1MOut           float64  `json:"cost_per_1m_out,omitempty"`
	CostPer1MInCached      float64  `json:"cost_per_1m_in_cached,omitempty"`
	CostPer1MOutCached     float64  `json:"cost_per_1m_out_cached,omitempty"`
}

type piManagedModel struct {
	Model piModel              `json:"model"`
	Route engineapi.ModelRoute `json:"route"`
}

// NewPiCatalog creates the host-side Pi catalog adapter. manifestPath stores
// which model entries this adapter last wrote to each workspace's engine config.
func NewPiCatalog(source interface {
	Load(context.Context) (modelcatalog.Catalog, error)
}, manifestPath string) *PiCatalog {
	return &PiCatalog{source: source, manifestPath: manifestPath}
}

// List returns Pi's supported model catalog, augmented only with runtime
// configuration from the engine. It also syncs the catalog models for
// configured providers into the engine before returning them.
func (p *PiCatalog) List(ctx context.Context, api *engineapi.Client, workspaceID string) ([]engineapi.Provider, error) {
	if p == nil || p.source == nil {
		return nil, errors.New("provider: Pi model catalog is unavailable")
	}
	if api == nil || strings.TrimSpace(workspaceID) == "" {
		return nil, errors.New("provider: engine API and workspace ID are required")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	catalog, err := p.source.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("load Pi model catalog: %w", err)
	}
	runtimeProviders, err := api.ListProviders(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list engine providers: %w", err)
	}
	config, err := api.GetWorkspaceConfig(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("get engine workspace config: %w", err)
	}
	manifest, err := p.readManifest()
	if err != nil {
		return nil, err
	}
	manifestBefore, _ := json.Marshal(manifest)
	workspaceManifest := manifest.workspace(workspaceID)

	runtimeByID := make(map[string]engineapi.Provider, len(runtimeProviders))
	for _, provider := range runtimeProviders {
		runtimeByID[normalizePiProviderID(provider.ID)] = provider
	}

	providerIDs := make([]string, 0, len(catalog))
	for id := range catalog {
		id = normalizePiProviderID(id)
		if id == CodexID { // Account-scoped Codex models come from the engine.
			continue
		}
		providerIDs = append(providerIDs, id)
	}
	slices.Sort(providerIDs)
	providerIDs = slices.Compact(providerIDs)

	providerModels := make(map[string][]engineapi.Model, len(providerIDs))
	providerTemplates := make(map[string]engineapi.Provider, len(providerIDs))
	updates := make(map[string]any)
	for _, providerID := range providerIDs {
		piModels := modelsForNormalizedProvider(catalog, providerID)
		runtimeProvider := runtimeByID[providerID]
		providerConfig, hasConfig := config.Providers[providerID]
		providerType := runtimeProvider.Type
		if providerType == "" {
			providerType = inferredPiType(providerID, piModels)
		}
		baseURL := providerConfig.BaseURL
		if baseURL == "" {
			baseURL = runtimeProvider.APIEndpoint
		}
		if baseURL == "" {
			baseURL = firstPiBaseURL(providerID, piModels)
		}

		customEndpoint := hasCustomPiEndpoint(providerConfig.BaseURL, runtimeProvider.APIEndpoint, firstPiBaseURL(providerID, piModels))
		desired, desiredRoutes := supportedPiModels(providerID, piModels, baseURL, customEndpoint)
		accountScopedOpenAI := providerID == OpenAIID && OAuthCredentialPresent(providerConfig)
		if accountScopedOpenAI {
			desired = nil
			desiredRoutes = nil
		}
		if hasConfig && !providerConfig.Disable && !accountScopedOpenAI {
			engineDefaults := runtimeProvider.Models
			merged, managed, routes := mergePiModels(providerConfig.Models, engineDefaults, workspaceManifest[providerID], desired, providerConfig.ModelRoutes, desiredRoutes, selectedModelIDs(config, providerID))
			providerModels[providerID] = merged
			workspaceManifest[providerID] = managed
			if !engineModelSlicesEqual(providerConfig.Models, merged) {
				updates["providers."+providerID+".models"] = wireModels(merged)
			}
			if !providerConfig.CatalogModels {
				updates["providers."+providerID+".catalog_models"] = true
			}
			if providerConfig.AutoDiscoverModels == nil || *providerConfig.AutoDiscoverModels {
				updates["providers."+providerID+".discover_models"] = false
				no := false
				providerConfig.AutoDiscoverModels = &no
			}
			if !modelRouteMapsEqual(providerConfig.ModelRoutes, routes) {
				updates["providers."+providerID+".model_routes"] = routes
			}
			providerConfig.Models = merged
			providerConfig.CatalogModels = true
			providerConfig.ModelRoutes = routes
			config.Providers[providerID] = providerConfig
		} else {
			providerModels[providerID] = mergePiModelsForDisplay(desired, providerConfig.Models)
		}

		name := runtimeProvider.Name
		if name == "" {
			name = providerID
		}
		entry := runtimeProvider
		entry.ID = providerID
		entry.Name = name
		entry.Type = providerType
		entry.APIEndpoint = baseURL
		entry.Models = providerModels[providerID]
		applyCredentialStatus(&entry, providerConfig, hasConfig)
		if len(entry.Models) == 0 && !entry.Configured {
			continue // Do not offer setup for providers with no callable Pi models.
		}
		setPiDefaults(&entry)
		providerTemplates[providerID] = entry
	}

	// Non-Pi providers are returned only when the workspace has explicit
	// configuration for them. This retains user-defined models and OAuth-only
	// providers without resurrecting Catwalk's removed default catalog entries.
	for id, providerConfig := range config.Providers {
		providerID := normalizePiProviderID(id)
		if _, exists := providerTemplates[providerID]; exists || providerConfig.Disable {
			continue
		}
		runtimeProvider := runtimeByID[providerID]
		if providerID != CodexID && len(providerConfig.Models) == 0 {
			continue
		}
		entry := runtimeProvider
		entry.ID = providerID
		entry.Name = firstNonEmpty(providerConfig.Name, runtimeProvider.Name, providerID)
		entry.Type = firstNonEmpty(runtimeProvider.Type, providerConfig.Type)
		entry.APIEndpoint = firstNonEmpty(providerConfig.BaseURL, runtimeProvider.APIEndpoint)
		entry.Models = cloneEngineModels(providerConfig.Models)
		if providerID != CodexID {
			entry.Models = explicitConfiguredModels(providerConfig.Models, runtimeProvider.Models, selectedModelIDs(config, providerID))
		}
		applyCredentialStatus(&entry, providerConfig, true)
		setPiDefaults(&entry)
		providerTemplates[providerID] = entry
	}
	if _, exists := providerTemplates[CodexID]; !exists {
		entry, hasRuntime := runtimeByID[CodexID]
		if !hasRuntime {
			entry = CodexSpec().Provider
		}
		if stored, ok := config.Providers[CodexID]; ok {
			if !stored.Disable {
				entry.Models = cloneEngineModels(stored.Models)
			} else {
				entry.Models = nil
				entry.Configured = false
			}
			entry.Name = firstNonEmpty(stored.Name, entry.Name)
			entry.Type = firstNonEmpty(stored.Type, entry.Type)
			entry.APIEndpoint = firstNonEmpty(stored.BaseURL, entry.APIEndpoint)
			applyCredentialStatus(&entry, stored, true)
		}
		setPiDefaults(&entry)
		providerTemplates[CodexID] = entry
	}

	if len(updates) > 0 {
		if err := api.SetConfigFields(ctx, workspaceID, engineapi.ConfigScopeWorkspace, updates); err != nil {
			return nil, fmt.Errorf("sync Pi models to engine config: %w", err)
		}
	}
	manifestAfter, _ := json.Marshal(manifest)
	if !reflect.DeepEqual(manifestBefore, manifestAfter) {
		if err := p.writeManifest(manifest); err != nil {
			return nil, err
		}
	}

	result := make([]engineapi.Provider, 0, len(providerTemplates))
	for _, provider := range providerTemplates {
		result = append(result, provider)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

// Prepare seeds a Pi provider before the caller saves credentials. Existing
// provider identity, endpoint, type, headers, and credentials are preserved.
func (p *PiCatalog) Prepare(ctx context.Context, api *engineapi.Client, workspaceID, providerID string) error {
	if p == nil || p.source == nil {
		return errors.New("provider: Pi model catalog is unavailable")
	}
	if api == nil || strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(providerID) == "" {
		return errors.New("provider: engine API, workspace ID, and provider ID are required")
	}
	providerID = normalizePiProviderID(providerID)
	if providerID == CodexID {
		return nil // Never seed public Pi models into an account-scoped catalog.
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	catalog, err := p.source.Load(ctx)
	if err != nil {
		return fmt.Errorf("load Pi model catalog: %w", err)
	}
	config, err := api.GetWorkspaceConfig(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("get engine workspace config: %w", err)
	}
	if providerID == OpenAIID && OAuthCredentialPresent(config.Providers[OpenAIID]) {
		return nil // Keep the legacy account catalog isolated from public Pi models.
	}
	runtimeProviders, err := api.ListProviders(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("list engine providers: %w", err)
	}
	runtimeProvider, _ := findProvider(runtimeProviders, providerID)
	providerConfig, exists := config.Providers[providerID]
	models := modelsForNormalizedProvider(catalog, providerID)
	if len(models) == 0 {
		return nil // Keep existing custom and engine-only provider setup flows.
	}
	providerType := firstNonEmpty(runtimeProvider.Type, providerConfig.Type, inferredPiType(providerID, models))
	catalogBaseURL := firstPiBaseURL(providerID, models)
	baseURL := firstNonEmpty(providerConfig.BaseURL, runtimeProvider.APIEndpoint, catalogBaseURL)
	customEndpoint := hasCustomPiEndpoint(providerConfig.BaseURL, runtimeProvider.APIEndpoint, catalogBaseURL)
	desired, desiredRoutes := supportedPiModels(providerID, models, baseURL, customEndpoint)
	if len(desired) == 0 {
		return nil // Unsupported Pi protocols remain available through custom setup.
	}

	manifest, err := p.readManifest()
	if err != nil {
		return err
	}
	workspaceManifest := manifest.workspace(workspaceID)
	engineDefaults := runtimeProvider.Models
	merged, managed, routes := mergePiModels(providerConfig.Models, engineDefaults, workspaceManifest[providerID], desired, providerConfig.ModelRoutes, desiredRoutes, selectedModelIDs(config, providerID))
	modelFields := map[string]any{
		"providers." + providerID + ".models":          wireModels(merged),
		"providers." + providerID + ".model_routes":    routes,
		"providers." + providerID + ".catalog_models":  true,
		"providers." + providerID + ".discover_models": false,
	}
	identityFields := map[string]any{}
	if !exists || providerConfig.Name == "" {
		identityFields["providers."+providerID+".name"] = firstNonEmpty(runtimeProvider.Name, providerID)
	}
	if !exists || providerConfig.Type == "" {
		identityFields["providers."+providerID+".type"] = providerType
	}
	if !exists || providerConfig.BaseURL == "" {
		identityFields["providers."+providerID+".base_url"] = baseURL
	}
	if len(identityFields) > 0 {
		if err := api.SetConfigFields(ctx, workspaceID, engineapi.ConfigScopeGlobal, identityFields); err != nil {
			return fmt.Errorf("prepare Pi provider %q identity in engine config: %w", providerID, err)
		}
	}
	if err := api.SetConfigFields(ctx, workspaceID, engineapi.ConfigScopeWorkspace, modelFields); err != nil {
		return fmt.Errorf("prepare Pi provider %q in engine config: %w", providerID, err)
	}
	workspaceManifest[providerID] = managed
	if err := p.writeManifest(manifest); err != nil {
		return err
	}
	return nil
}

func (m *piCatalogManifest) workspace(workspaceID string) map[string]map[string]piManagedModel {
	if m.Workspaces == nil {
		m.Workspaces = make(map[string]map[string]map[string]piManagedModel)
	}
	if m.Workspaces[workspaceID] == nil {
		m.Workspaces[workspaceID] = make(map[string]map[string]piManagedModel)
	}
	return m.Workspaces[workspaceID]
}

func (p *PiCatalog) readManifest() (*piCatalogManifest, error) {
	manifest := &piCatalogManifest{Version: 1, Workspaces: make(map[string]map[string]map[string]piManagedModel)}
	if p.manifestPath == "" {
		return manifest, nil
	}
	data, err := os.ReadFile(p.manifestPath)
	if errors.Is(err, os.ErrNotExist) {
		return manifest, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read Pi model manifest: %w", err)
	}
	if err := json.Unmarshal(data, manifest); err != nil {
		return nil, fmt.Errorf("decode Pi model manifest: %w", err)
	}
	if manifest.Version != 1 {
		return nil, fmt.Errorf("unsupported Pi model manifest version %d", manifest.Version)
	}
	if manifest.Workspaces == nil {
		manifest.Workspaces = make(map[string]map[string]map[string]piManagedModel)
	}
	return manifest, nil
}

func (p *PiCatalog) writeManifest(manifest *piCatalogManifest) error {
	if p.manifestPath == "" {
		return nil
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("encode Pi model manifest: %w", err)
	}
	directory := filepath.Dir(p.manifestPath)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create Pi model manifest directory: %w", err)
	}
	file, err := os.CreateTemp(directory, ".pi-model-manifest-*.tmp")
	if err != nil {
		return fmt.Errorf("create Pi model manifest: %w", err)
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return fmt.Errorf("set Pi model manifest permissions: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("write Pi model manifest: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync Pi model manifest: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close Pi model manifest: %w", err)
	}
	if err := os.Rename(tempPath, p.manifestPath); err != nil {
		return fmt.Errorf("replace Pi model manifest: %w", err)
	}
	return nil
}

func normalizePiProviderID(providerID string) string {
	switch providerID {
	case "openai-codex":
		return CodexID
	case "github-copilot":
		return "copilot"
	case "amazon-bedrock":
		return "bedrock"
	case "google-vertex":
		return "vertexai"
	case "minimax-cn":
		return "minimax-china"
	case "moonshotai":
		return "moonshot"
	case "vercel-ai-gateway":
		return "vercel"
	default:
		return providerID
	}
}

func modelsForNormalizedProvider(catalog modelcatalog.Catalog, providerID string) []modelcatalog.Model {
	var models []modelcatalog.Model
	for id, providerModels := range catalog {
		if normalizePiProviderID(id) != providerID {
			continue
		}
		for key, model := range providerModels {
			if model.ID == "" {
				model.ID = key
			}
			if model.Provider == "" {
				model.Provider = id
			}
			models = append(models, model)
		}
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return models
}

func inferredPiType(providerID string, models []modelcatalog.Model) string {
	for _, model := range models {
		if typ := engineTypeForPiAPI(canonicalPiAPI(providerID, model.API)); typ != "" {
			return typ
		}
	}
	return ""
}

func engineTypeForPiAPI(api string) string {
	switch api {
	case "openai-completions":
		return OpenAICompatType
	case "openai-responses":
		return "openai"
	case "anthropic-messages":
		return "anthropic"
	case "google-generative-ai":
		return "google"
	case "google-vertex":
		return "google-vertex"
	case "azure-openai-responses":
		return "azure"
	case "bedrock-converse-stream":
		return "bedrock"
	default:
		return ""
	}
}

func canonicalPiAPI(providerID, api string) string {
	if providerID == "mistral" && api == "mistral-conversations" {
		return "openai-completions"
	}
	switch api {
	case "openai-completions", "openai-responses", "anthropic-messages", "google-generative-ai", "google-vertex", "azure-openai-responses", "bedrock-converse-stream":
		return api
	default:
		return ""
	}
}

func firstPiBaseURL(providerID string, models []modelcatalog.Model) string {
	for _, model := range models {
		if canonicalPiAPI(providerID, model.API) != "" && model.BaseURL != "" {
			return normalizePiBaseURL(providerID, model.BaseURL)
		}
	}
	return ""
}

func normalizePiBaseURL(providerID, baseURL string) string {
	if normalizePiProviderID(providerID) == "mistral" && strings.TrimRight(baseURL, "/") == "https://api.mistral.ai" {
		return "https://api.mistral.ai/v1"
	}
	return baseURL
}

func supportedPiModels(providerID string, models []modelcatalog.Model, providerBaseURL string, customEndpoint bool) ([]engineapi.Model, map[string]engineapi.ModelRoute) {
	var supported []engineapi.Model
	routes := make(map[string]engineapi.ModelRoute)
	for _, model := range models {
		if model.Enabled != nil && !*model.Enabled {
			continue
		}
		if model.Type != "" && model.Type != "chat" {
			continue
		}
		if len(model.Input) > 0 && !slices.Contains(model.Input, "text") {
			continue
		}
		api := canonicalPiAPI(providerID, model.API)
		if api == "" || model.ID == "" || api == "openai-codex-responses" {
			continue
		}
		routeURL := ""
		modelBaseURL := normalizePiBaseURL(providerID, model.BaseURL)
		if !customEndpoint && modelBaseURL != "" && providerBaseURL != "" && !sameBaseURL(providerBaseURL, modelBaseURL) {
			routeURL = modelBaseURL
		}
		supported = append(supported, engineapi.Model{
			ID:                     model.ID,
			Name:                   firstNonEmpty(model.Name, model.ID),
			ContextWindow:          model.ContextWindow,
			DefaultMaxTokens:       model.MaxTokens,
			CanReason:              model.Reasoning,
			SupportsVision:         slices.Contains(model.Input, "image"),
			Modalities:             slices.Clone(model.Input),
			ReasoningLevels:        piReasoningLevels(model),
			DefaultReasoningEffort: piDefaultReasoningEffort(model),
			CostPer1MIn:            model.Cost.Input,
			CostPer1MOut:           model.Cost.Output,
			CostPer1MInCached:      model.Cost.CacheRead,
			CostPer1MOutCached:     model.Cost.CacheWrite,
		})
		routes[model.ID] = engineapi.ModelRoute{API: api, BaseURL: routeURL, Headers: cloneStringMap(model.Headers)}
	}
	sort.Slice(supported, func(i, j int) bool { return supported[i].ID < supported[j].ID })
	return supported, routes
}

func piReasoningLevels(model modelcatalog.Model) []string {
	levels := make([]string, 0, len(model.ThinkingLevelMap))
	for level, value := range model.ThinkingLevelMap {
		if level == "off" {
			levels = append(levels, "none")
			continue
		}
		if !slices.Contains([]string{"minimal", "low", "medium", "high", "xhigh", "max"}, level) {
			continue
		}
		mapped := strings.Trim(strings.TrimSpace(string(value)), `"`)
		if mapped != level {
			continue
		}
		levels = append(levels, level)
	}
	order := []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"}
	sort.Slice(levels, func(i, j int) bool {
		return slices.Index(order, levels[i]) < slices.Index(order, levels[j])
	})
	return levels
}

func piDefaultReasoningEffort(model modelcatalog.Model) string {
	if !model.Reasoning {
		return ""
	}
	levels := piReasoningLevels(model)
	if slices.Contains(levels, "medium") {
		return "medium"
	}
	for _, level := range levels {
		if level != "none" {
			return level
		}
	}
	return ""
}

func mergePiModels(current, engineDefaults []engineapi.Model, previous map[string]piManagedModel, desired []engineapi.Model, currentRoutes, desiredRoutes map[string]engineapi.ModelRoute, selected map[string]bool) ([]engineapi.Model, map[string]piManagedModel, map[string]engineapi.ModelRoute) {
	wanted := make(map[string]engineapi.Model, len(desired))
	for _, model := range desired {
		wanted[model.ID] = model
	}
	defaultByID := make(map[string]engineapi.Model, len(engineDefaults))
	for _, model := range engineDefaults {
		defaultByID[model.ID] = model
	}
	merged := make([]engineapi.Model, 0, len(current)+len(desired))
	managed := make(map[string]piManagedModel, len(desired))
	routes := make(map[string]engineapi.ModelRoute, len(currentRoutes)+len(desiredRoutes))
	seen := make(map[string]bool, len(current)+len(desired))
	for _, model := range current {
		if model.ID == "" || seen[model.ID] {
			continue
		}
		seen[model.ID] = true
		prior, piManaged := previous[model.ID]
		if piManaged && engineModelEqual(model, piModelToEngineModel(prior.Model)) && modelRouteEqual(currentRoutes[model.ID], prior.Route) {
			if replacement, stillAvailable := wanted[model.ID]; stillAvailable {
				merged = append(merged, replacement)
				managed[model.ID] = piManagedModel{Model: piModelFromEngineModel(replacement), Route: cloneModelRoute(desiredRoutes[model.ID])}
				routes[model.ID] = desiredRoutes[model.ID]
			} else if selected[model.ID] {
				merged = append(merged, model)
				managed[model.ID] = prior
				if route, ok := currentRoutes[model.ID]; ok {
					routes[model.ID] = route
				}
			}
			continue
		}
		if !piManaged {
			if defaultModel, isDefault := defaultByID[model.ID]; isDefault && engineModelEqual(model, defaultModel) {
				if replacement, stillAvailable := wanted[model.ID]; stillAvailable {
					merged = append(merged, replacement)
					managed[model.ID] = piManagedModel{Model: piModelFromEngineModel(replacement), Route: cloneModelRoute(desiredRoutes[model.ID])}
					routes[model.ID] = desiredRoutes[model.ID]
				} else if selected[model.ID] {
					merged = append(merged, model)
					managed[model.ID] = piManagedModel{Model: piModelFromEngineModel(model), Route: cloneModelRoute(currentRoutes[model.ID])}
					if route, ok := currentRoutes[model.ID]; ok {
						routes[model.ID] = route
					}
				}
				continue
			}
		}
		merged = append(merged, model)
		if route, ok := currentRoutes[model.ID]; ok {
			routes[model.ID] = route
		}
	}
	for _, model := range desired {
		if seen[model.ID] {
			continue
		}
		seen[model.ID] = true
		merged = append(merged, model)
		managed[model.ID] = piManagedModel{Model: piModelFromEngineModel(model), Route: cloneModelRoute(desiredRoutes[model.ID])}
		routes[model.ID] = desiredRoutes[model.ID]
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].ID < merged[j].ID })
	return merged, managed, routes
}

func selectedModelIDs(config engineapi.WorkspaceConfig, providerID string) map[string]bool {
	selected := make(map[string]bool)
	for _, model := range config.Models {
		if normalizePiProviderID(model.Provider) == providerID && model.Model != "" {
			selected[model.Model] = true
		}
	}
	return selected
}

func engineModelSlicesEqual(left, right []engineapi.Model) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if !engineModelEqual(left[i], right[i]) {
			return false
		}
	}
	return true
}

func modelRouteMapsEqual(left, right map[string]engineapi.ModelRoute) bool {
	if len(left) != len(right) {
		return false
	}
	for id, route := range left {
		other, ok := right[id]
		if !ok || !modelRouteEqual(route, other) {
			return false
		}
	}
	return true
}

func mergePiModelsForDisplay(desired, configured []engineapi.Model) []engineapi.Model {
	merged := cloneEngineModels(desired)
	seen := make(map[string]bool, len(merged)+len(configured))
	for _, model := range merged {
		seen[model.ID] = true
	}
	for _, model := range configured {
		if model.ID == "" || seen[model.ID] {
			continue
		}
		seen[model.ID] = true
		merged = append(merged, model)
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].ID < merged[j].ID })
	return merged
}

func explicitConfiguredModels(configured, engineDefaults []engineapi.Model, selected map[string]bool) []engineapi.Model {
	defaults := make(map[string]engineapi.Model, len(engineDefaults))
	for _, model := range engineDefaults {
		defaults[model.ID] = model
	}
	result := make([]engineapi.Model, 0, len(configured))
	for _, model := range configured {
		if model.ID == "" {
			continue
		}
		if defaultModel, isDefault := defaults[model.ID]; isDefault && engineModelEqual(model, defaultModel) && !selected[model.ID] {
			continue
		}
		result = append(result, model)
	}
	return result
}

func wireModels(models []engineapi.Model) []piModel {
	result := make([]piModel, 0, len(models))
	for _, model := range models {
		result = append(result, piModelFromEngineModel(model))
	}
	return result
}

func piModelFromEngineModel(model engineapi.Model) piModel {
	return piModel{
		ID:                     model.ID,
		Name:                   model.Name,
		ContextWindow:          model.ContextWindow,
		DefaultMaxTokens:       model.DefaultMaxTokens,
		CanReason:              model.CanReason,
		SupportsAttachments:    model.SupportsVision,
		ReasoningLevels:        slices.Clone(model.ReasoningLevels),
		DefaultReasoningEffort: model.DefaultReasoningEffort,
		CostPer1MIn:            model.CostPer1MIn,
		CostPer1MOut:           model.CostPer1MOut,
		CostPer1MInCached:      model.CostPer1MInCached,
		CostPer1MOutCached:     model.CostPer1MOutCached,
	}
}

func piModelToEngineModel(model piModel) engineapi.Model {
	return engineapi.Model{
		ID:                     model.ID,
		Name:                   model.Name,
		ContextWindow:          model.ContextWindow,
		DefaultMaxTokens:       model.DefaultMaxTokens,
		CanReason:              model.CanReason,
		SupportsVision:         model.SupportsAttachments,
		ReasoningLevels:        slices.Clone(model.ReasoningLevels),
		DefaultReasoningEffort: model.DefaultReasoningEffort,
		CostPer1MIn:            model.CostPer1MIn,
		CostPer1MOut:           model.CostPer1MOut,
		CostPer1MInCached:      model.CostPer1MInCached,
		CostPer1MOutCached:     model.CostPer1MOutCached,
	}
}

func engineModelEqual(left, right engineapi.Model) bool {
	left.Modalities = nil
	right.Modalities = nil
	return left.ID == right.ID && left.Name == right.Name && left.ContextWindow == right.ContextWindow &&
		left.DefaultMaxTokens == right.DefaultMaxTokens && left.CanReason == right.CanReason &&
		left.SupportsVision == right.SupportsVision && slices.Equal(left.ReasoningLevels, right.ReasoningLevels) &&
		left.DefaultReasoningEffort == right.DefaultReasoningEffort && math.Abs(left.CostPer1MIn-right.CostPer1MIn) < 1e-12 &&
		math.Abs(left.CostPer1MOut-right.CostPer1MOut) < 1e-12 &&
		math.Abs(left.CostPer1MInCached-right.CostPer1MInCached) < 1e-12 &&
		math.Abs(left.CostPer1MOutCached-right.CostPer1MOutCached) < 1e-12
}

func modelRouteEqual(left, right engineapi.ModelRoute) bool {
	if left.API != right.API || !sameBaseURL(left.BaseURL, right.BaseURL) || len(left.Headers) != len(right.Headers) {
		return false
	}
	for key, value := range left.Headers {
		if right.Headers[key] != value {
			return false
		}
	}
	return true
}

func cloneModelRoute(route engineapi.ModelRoute) engineapi.ModelRoute {
	return engineapi.ModelRoute{API: route.API, BaseURL: route.BaseURL, Headers: cloneStringMap(route.Headers)}
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func hasCustomPiEndpoint(configuredURL, runtimeURL, piURL string) bool {
	if configuredURL == "" {
		return false
	}
	_ = runtimeURL // The runtime endpoint may itself be a user override.
	return piURL == "" || !sameBaseURL(configuredURL, piURL)
}

func cloneEngineModels(models []engineapi.Model) []engineapi.Model {
	cloned := slices.Clone(models)
	for i := range cloned {
		cloned[i].Modalities = slices.Clone(cloned[i].Modalities)
		cloned[i].ReasoningLevels = slices.Clone(cloned[i].ReasoningLevels)
	}
	return cloned
}

func setPiDefaults(provider *engineapi.Provider) {
	if len(provider.Models) == 0 {
		provider.DefaultLargeModelID = ""
		provider.DefaultSmallModelID = ""
		return
	}
	contains := func(id string) bool {
		return id != "" && slices.ContainsFunc(provider.Models, func(model engineapi.Model) bool { return model.ID == id })
	}
	if !contains(provider.DefaultLargeModelID) {
		provider.DefaultLargeModelID = provider.Models[0].ID
	}
	if !contains(provider.DefaultSmallModelID) {
		provider.DefaultSmallModelID = provider.Models[0].ID
	}
}

func findProvider(providers []engineapi.Provider, providerID string) (engineapi.Provider, bool) {
	for _, provider := range providers {
		if normalizePiProviderID(provider.ID) == providerID {
			return provider, true
		}
	}
	return engineapi.Provider{}, false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func applyCredentialStatus(entry *engineapi.Provider, config engineapi.ProviderConfig, exists bool) {
	if !exists {
		return // Keep any status supplied by an engine/runtime that knows better.
	}
	entry.Configured = false
	entry.CredentialKind = ""
	if config.Disable {
		return
	}
	kind, _, usable := ResolvedCredential(config)
	entry.Configured = usable
	if usable {
		entry.CredentialKind = kind
	}
}

func sameBaseURL(left, right string) bool {
	return strings.TrimRight(strings.TrimSpace(left), "/") == strings.TrimRight(strings.TrimSpace(right), "/")
}
