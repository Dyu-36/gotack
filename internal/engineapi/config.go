package engineapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

type WorkspaceConfig struct {
	Providers map[string]ProviderConfig `json:"providers,omitempty"`
	Models    map[string]SelectedModel  `json:"models,omitempty"`
	Options   *WorkspaceOptions         `json:"options,omitempty"`
	Env       map[string]string         `json:"env,omitempty"`

	Hooks map[string][]HookEntry `json:"hooks,omitempty"`
}

type WorkspaceOptions struct {
	SkillsPaths        []string `json:"skills_paths,omitempty"`
	GlobalContextPaths []string `json:"global_context_paths,omitempty"`
}

func (c WorkspaceConfig) SkillsPaths() []string {
	if c.Options == nil {
		return nil
	}
	return c.Options.SkillsPaths
}

type HookEntry struct {
	Name    string `json:"name,omitempty"`
	Matcher string `json:"matcher,omitempty"`
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

type ProviderConfig struct {
	ID      string          `json:"id,omitempty"`
	Name    string          `json:"name,omitempty"`
	BaseURL string          `json:"base_url,omitempty"`
	Type    string          `json:"type,omitempty"`
	APIKey  string          `json:"api_key,omitempty"`
	OAuth   json.RawMessage `json:"oauth,omitempty"`
	Disable bool            `json:"disable,omitempty"`
	Models  []Model         `json:"models,omitempty"`
}

func (c *Client) GetWorkspaceConfig(ctx context.Context, wsID string) (WorkspaceConfig, error) {
	if wsID == "" {
		return WorkspaceConfig{}, errors.New("engineapi: workspace id is required")
	}
	var cfg WorkspaceConfig
	if err := c.doJSON(ctx, "GET", expandPath("/v1/workspaces/{id}/config", "id", wsID), nil, &cfg); err != nil {
		return WorkspaceConfig{}, err
	}
	return cfg, nil
}

const (
	configSetPath          = "/v1/workspaces/{id}/config/set"
	configSetBatchPath     = "/v1/workspaces/{id}/config/set-batch"
	configRemovePath       = "/v1/workspaces/{id}/config/remove"
	configModelPath        = "/v1/workspaces/{id}/config/model"
	configModelsPath       = "/v1/workspaces/{id}/config/models"
	configProviderKeyPath  = "/v1/workspaces/{id}/config/provider-key"
	configRefreshOAuthPath = "/v1/workspaces/{id}/config/refresh-oauth"
)

const (
	ConfigScopeGlobal    = 0
	ConfigScopeWorkspace = 1
)

var preferredModelTypes = [...]string{"large", "small"}

type SelectedModel struct {
	Model           string `json:"model"`
	Provider        string `json:"provider"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	Think           bool   `json:"think,omitempty"`
}

func (c *Client) SetPreferredModelPair(ctx context.Context, wsID string, scope int, model SelectedModel) error {
	if wsID == "" {
		return errors.New("engineapi: workspace id is required")
	}
	if strings.TrimSpace(model.Provider) == "" || strings.TrimSpace(model.Model) == "" {
		return errors.New("engineapi: provider and model are required")
	}
	return c.mutatePreferredModelPair(ctx, wsID, scope, &model)
}

func (c *Client) RemovePreferredModelPair(ctx context.Context, wsID string, scope int) error {
	if wsID == "" {
		return errors.New("engineapi: workspace id is required")
	}
	return c.mutatePreferredModelPair(ctx, wsID, scope, nil)
}

func (c *Client) mutatePreferredModelPair(ctx context.Context, wsID string, scope int, model *SelectedModel) error {
	body, err := json.Marshal(struct {
		Scope  int                       `json:"scope"`
		Models map[string]*SelectedModel `json:"models"`
	}{
		Scope: scope,
		Models: map[string]*SelectedModel{
			"large": model,
			"small": model,
		},
	})
	if err != nil {
		return fmt.Errorf("engineapi: encode preferred model pair: %w", err)
	}

	err = c.doJSON(ctx, http.MethodPost, expandPath(configModelsPath, "id", wsID), bytes.NewReader(body), nil)
	if err == nil || !isHTTPStatus(err, http.StatusNotFound) {
		return err
	}

	return c.mutatePreferredModelPairLegacy(ctx, wsID, scope, model)
}

func (c *Client) mutatePreferredModelPairLegacy(ctx context.Context, wsID string, scope int, model *SelectedModel) error {
	for _, modelType := range preferredModelTypes {
		if model == nil {
			if err := c.RemoveConfigField(ctx, wsID, scope, "models."+modelType); err != nil {
				return fmt.Errorf("remove %s preferred model through legacy engine API: %w", modelType, err)
			}
			continue
		}

		body, err := json.Marshal(struct {
			Scope     int           `json:"scope"`
			ModelType string        `json:"model_type"`
			Model     SelectedModel `json:"model"`
		}{Scope: scope, ModelType: modelType, Model: *model})
		if err != nil {
			return fmt.Errorf("engineapi: encode %s preferred model: %w", modelType, err)
		}
		if err := c.doJSON(ctx, http.MethodPost, expandPath(configModelPath, "id", wsID), bytes.NewReader(body), nil); err != nil {
			return fmt.Errorf("update %s preferred model through legacy engine API: %w", modelType, err)
		}
	}
	return nil
}

func (c *Client) SetProviderAPIKey(ctx context.Context, wsID string, scope int, providerID, apiKey string) error {
	if wsID == "" || strings.TrimSpace(providerID) == "" {
		return errors.New("engineapi: workspace id and provider id are required")
	}
	body, err := encodeProviderKeyRequest(scope, providerID, "string", apiKey)
	if err != nil {
		return fmt.Errorf("engineapi: encode provider key request: %w", err)
	}
	return c.doJSON(ctx, http.MethodPost, expandPath(configProviderKeyPath, "id", wsID), bytes.NewReader(body), nil)
}

func (c *Client) SetProviderOAuthToken(ctx context.Context, wsID string, scope int, providerID string, token any) error {
	if wsID == "" || strings.TrimSpace(providerID) == "" {
		return errors.New("engineapi: workspace id and provider id are required")
	}
	body, err := encodeProviderKeyRequest(scope, providerID, "oauth", token)
	if err != nil {
		return fmt.Errorf("engineapi: encode provider oauth token: %w", err)
	}
	return c.doJSON(ctx, http.MethodPost, expandPath(configProviderKeyPath, "id", wsID), bytes.NewReader(body), nil)
}

func encodeProviderKeyRequest(scope int, providerID, kind string, value any) ([]byte, error) {
	return json.Marshal(struct {
		Scope      int    `json:"scope"`
		ProviderID string `json:"provider_id"`
		Kind       string `json:"kind"`
		APIKey     any    `json:"api_key"`
	}{Scope: scope, ProviderID: providerID, Kind: kind, APIKey: value})
}

func (c *Client) RefreshProviderOAuthToken(ctx context.Context, wsID string, scope int, providerID string) error {
	if wsID == "" || strings.TrimSpace(providerID) == "" {
		return errors.New("engineapi: workspace id and provider id are required")
	}
	body, err := json.Marshal(struct {
		Scope      int    `json:"scope"`
		ProviderID string `json:"provider_id"`
	}{Scope: scope, ProviderID: providerID})
	if err != nil {
		return fmt.Errorf("engineapi: encode provider oauth refresh: %w", err)
	}
	return c.doJSON(ctx, http.MethodPost, expandPath(configRefreshOAuthPath, "id", wsID), bytes.NewReader(body), nil)
}

func (c *Client) SetConfigField(ctx context.Context, wsID string, scope int, key string, value any) error {
	if wsID == "" || strings.TrimSpace(key) == "" {
		return errors.New("engineapi: workspace id and config key are required")
	}
	body, err := json.Marshal(struct {
		Scope int    `json:"scope"`
		Key   string `json:"key"`
		Value any    `json:"value"`
	}{Scope: scope, Key: key, Value: value})
	if err != nil {
		return fmt.Errorf("engineapi: encode config field: %w", err)
	}
	return c.doJSON(ctx, http.MethodPost, expandPath(configSetPath, "id", wsID), bytes.NewReader(body), nil)
}

func (c *Client) SetConfigFields(ctx context.Context, wsID string, scope int, fields map[string]any) error {
	if wsID == "" {
		return errors.New("engineapi: workspace id is required")
	}
	if len(fields) == 0 {
		return errors.New("engineapi: config fields are required")
	}
	for key := range fields {
		if strings.TrimSpace(key) == "" {
			return errors.New("engineapi: config field key is required")
		}
	}
	body, err := json.Marshal(struct {
		Scope  int            `json:"scope"`
		Fields map[string]any `json:"fields"`
	}{Scope: scope, Fields: fields})
	if err != nil {
		return fmt.Errorf("engineapi: encode config fields: %w", err)
	}

	err = c.doJSON(ctx, http.MethodPost, expandPath(configSetBatchPath, "id", wsID), bytes.NewReader(body), nil)
	if err == nil || !isHTTPStatus(err, http.StatusNotFound) {
		return err
	}

	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := c.SetConfigField(ctx, wsID, scope, key, fields[key]); err != nil {
			return fmt.Errorf("set config field %q through legacy engine API: %w", key, err)
		}
	}
	return nil
}

func (c *Client) RemoveConfigField(ctx context.Context, wsID string, scope int, key string) error {
	if wsID == "" || strings.TrimSpace(key) == "" {
		return errors.New("engineapi: workspace id and config key are required")
	}
	body, err := json.Marshal(struct {
		Scope int    `json:"scope"`
		Key   string `json:"key"`
	}{Scope: scope, Key: key})
	if err != nil {
		return fmt.Errorf("engineapi: encode config removal: %w", err)
	}
	return c.doJSON(ctx, http.MethodPost, expandPath(configRemovePath, "id", wsID), bytes.NewReader(body), nil)
}

func isHTTPStatus(err error, want int) bool {
	for err != nil {
		text, ok := strings.CutPrefix(err.Error(), "engineapi: ")
		if ok {
			if _, statusText, found := strings.Cut(text, ": "); found {
				codeText, _, _ := strings.Cut(statusText, " ")
				if code, convErr := strconv.Atoi(codeText); convErr == nil && code == want {
					return true
				}
			}
		}
		err = errors.Unwrap(err)
	}
	return false
}
