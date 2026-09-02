package crushapi

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
		return errors.New("crushapi: workspace id is required")
	}
	if strings.TrimSpace(model.Provider) == "" || strings.TrimSpace(model.Model) == "" {
		return errors.New("crushapi: provider and model are required")
	}
	return c.mutatePreferredModelPair(ctx, wsID, scope, &model)
}

func (c *Client) RemovePreferredModelPair(ctx context.Context, wsID string, scope int) error {
	if wsID == "" {
		return errors.New("crushapi: workspace id is required")
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
		return fmt.Errorf("crushapi: encode preferred model pair: %w", err)
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
				return fmt.Errorf("remove %s preferred model through legacy Crush API: %w", modelType, err)
			}
			continue
		}

		body, err := json.Marshal(struct {
			Scope     int           `json:"scope"`
			ModelType string        `json:"model_type"`
			Model     SelectedModel `json:"model"`
		}{Scope: scope, ModelType: modelType, Model: *model})
		if err != nil {
			return fmt.Errorf("crushapi: encode %s preferred model: %w", modelType, err)
		}
		if err := c.doJSON(ctx, http.MethodPost, expandPath(configModelPath, "id", wsID), bytes.NewReader(body), nil); err != nil {
			return fmt.Errorf("update %s preferred model through legacy Crush API: %w", modelType, err)
		}
	}
	return nil
}

func (c *Client) SetProviderAPIKey(ctx context.Context, wsID string, scope int, providerID, apiKey string) error {
	if wsID == "" || strings.TrimSpace(providerID) == "" {
		return errors.New("crushapi: workspace id and provider id are required")
	}
	raw, err := json.Marshal(apiKey)
	if err != nil {
		return fmt.Errorf("crushapi: encode provider key: %w", err)
	}
	body, err := json.Marshal(struct {
		Scope      int             `json:"scope"`
		ProviderID string          `json:"provider_id"`
		Kind       string          `json:"kind"`
		APIKey     json.RawMessage `json:"api_key"`
	}{Scope: scope, ProviderID: providerID, Kind: "string", APIKey: raw})
	if err != nil {
		return fmt.Errorf("crushapi: encode provider key request: %w", err)
	}
	return c.doJSON(ctx, http.MethodPost, expandPath(configProviderKeyPath, "id", wsID), bytes.NewReader(body), nil)
}

func (c *Client) SetProviderOAuthToken(ctx context.Context, wsID string, scope int, providerID string, token any) error {
	if wsID == "" || strings.TrimSpace(providerID) == "" {
		return errors.New("crushapi: workspace id and provider id are required")
	}
	raw, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("crushapi: encode provider oauth token: %w", err)
	}
	body, err := json.Marshal(struct {
		Scope      int             `json:"scope"`
		ProviderID string          `json:"provider_id"`
		Kind       string          `json:"kind"`
		APIKey     json.RawMessage `json:"api_key"`
	}{Scope: scope, ProviderID: providerID, Kind: "oauth", APIKey: raw})
	if err != nil {
		return fmt.Errorf("crushapi: encode provider oauth key request: %w", err)
	}
	return c.doJSON(ctx, http.MethodPost, expandPath(configProviderKeyPath, "id", wsID), bytes.NewReader(body), nil)
}

func (c *Client) RefreshProviderOAuthToken(ctx context.Context, wsID string, scope int, providerID string) error {
	if wsID == "" || strings.TrimSpace(providerID) == "" {
		return errors.New("crushapi: workspace id and provider id are required")
	}
	body, err := json.Marshal(struct {
		Scope      int    `json:"scope"`
		ProviderID string `json:"provider_id"`
	}{Scope: scope, ProviderID: providerID})
	if err != nil {
		return fmt.Errorf("crushapi: encode provider oauth refresh: %w", err)
	}
	return c.doJSON(ctx, http.MethodPost, expandPath(configRefreshOAuthPath, "id", wsID), bytes.NewReader(body), nil)
}

func (c *Client) SetConfigField(ctx context.Context, wsID string, scope int, key string, value any) error {
	if wsID == "" || strings.TrimSpace(key) == "" {
		return errors.New("crushapi: workspace id and config key are required")
	}
	body, err := json.Marshal(struct {
		Scope int    `json:"scope"`
		Key   string `json:"key"`
		Value any    `json:"value"`
	}{Scope: scope, Key: key, Value: value})
	if err != nil {
		return fmt.Errorf("crushapi: encode config field: %w", err)
	}
	return c.doJSON(ctx, http.MethodPost, expandPath(configSetPath, "id", wsID), bytes.NewReader(body), nil)
}

func (c *Client) SetConfigFields(ctx context.Context, wsID string, scope int, fields map[string]any) error {
	if wsID == "" {
		return errors.New("crushapi: workspace id is required")
	}
	if len(fields) == 0 {
		return errors.New("crushapi: config fields are required")
	}
	for key := range fields {
		if strings.TrimSpace(key) == "" {
			return errors.New("crushapi: config field key is required")
		}
	}
	body, err := json.Marshal(struct {
		Scope  int            `json:"scope"`
		Fields map[string]any `json:"fields"`
	}{Scope: scope, Fields: fields})
	if err != nil {
		return fmt.Errorf("crushapi: encode config fields: %w", err)
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
			return fmt.Errorf("set config field %q through legacy Crush API: %w", key, err)
		}
	}
	return nil
}

func (c *Client) RemoveConfigField(ctx context.Context, wsID string, scope int, key string) error {
	if wsID == "" || strings.TrimSpace(key) == "" {
		return errors.New("crushapi: workspace id and config key are required")
	}
	body, err := json.Marshal(struct {
		Scope int    `json:"scope"`
		Key   string `json:"key"`
	}{Scope: scope, Key: key})
	if err != nil {
		return fmt.Errorf("crushapi: encode config removal: %w", err)
	}
	return c.doJSON(ctx, http.MethodPost, expandPath(configRemovePath, "id", wsID), bytes.NewReader(body), nil)
}

func isHTTPStatus(err error, want int) bool {
	for err != nil {
		text, ok := strings.CutPrefix(err.Error(), "crushapi: ")
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
