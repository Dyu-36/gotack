package crushapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type configMutationRoundTripper func(*http.Request) (*http.Response, error)

func (f configMutationRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func emptyHTTPResponse(status int) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("")),
	}
}

func TestSetConfigFieldsPostsAtomicBatch(t *testing.T) {
	var got struct {
		Scope  int                        `json:"scope"`
		Fields map[string]json.RawMessage `json:"fields"`
	}
	client := NewClient(&http.Client{Transport: configMutationRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/workspaces/ws-1/config/set-batch" {
			t.Fatalf("request = %s %s", req.Method, req.URL.Path)
		}
		if err := json.NewDecoder(req.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		return emptyHTTPResponse(http.StatusOK), nil
	})})

	err := client.SetConfigFields(context.Background(), "ws-1", ConfigScopeGlobal, map[string]any{
		"providers.mistral.name":            "Mistral AI",
		"providers.mistral.discover_models": false,
	})
	if err != nil {
		t.Fatalf("SetConfigFields() error = %v", err)
	}
	if got.Scope != ConfigScopeGlobal || string(got.Fields["providers.mistral.name"]) != `"Mistral AI"` || string(got.Fields["providers.mistral.discover_models"]) != "false" {
		t.Fatalf("batch payload = %#v", got)
	}
}

func TestPreferredModelPairUsesOneAtomicRequest(t *testing.T) {
	var got struct {
		Scope  int                         `json:"scope"`
		Models map[string]*json.RawMessage `json:"models"`
	}
	requests := 0
	client := NewClient(&http.Client{Transport: configMutationRoundTripper(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Method != http.MethodPost || req.URL.Path != "/v1/workspaces/ws-1/config/models" {
			t.Fatalf("request = %s %s", req.Method, req.URL.Path)
		}
		if err := json.NewDecoder(req.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		return emptyHTTPResponse(http.StatusOK), nil
	})})

	selected := SelectedModel{Provider: "anthropic", Model: "claude", ReasoningEffort: "high", Think: true}
	if err := client.SetPreferredModelPair(context.Background(), "ws-1", ConfigScopeGlobal, selected); err != nil {
		t.Fatalf("SetPreferredModelPair() error = %v", err)
	}
	if requests != 1 || len(got.Models) != 2 || got.Models["large"] == nil || got.Models["small"] == nil {
		t.Fatalf("set payload = %#v, requests = %d", got, requests)
	}
	for _, modelType := range []string{"large", "small"} {
		var model SelectedModel
		if err := json.Unmarshal(*got.Models[modelType], &model); err != nil {
			t.Fatalf("decode %s model: %v", modelType, err)
		}
		if model != selected {
			t.Fatalf("%s model = %#v, want %#v", modelType, model, selected)
		}
	}

	got.Models = nil
	if err := client.RemovePreferredModelPair(context.Background(), "ws-1", ConfigScopeGlobal); err != nil {
		t.Fatalf("RemovePreferredModelPair() error = %v", err)
	}
	if requests != 2 || len(got.Models) != 2 || got.Models["large"] != nil || got.Models["small"] != nil {
		t.Fatalf("remove payload = %#v, requests = %d", got, requests)
	}
}

func TestAtomicConfigMutationsRejectInvalidInputBeforeRequest(t *testing.T) {
	requests := 0
	client := NewClient(&http.Client{Transport: configMutationRoundTripper(func(*http.Request) (*http.Response, error) {
		requests++
		return emptyHTTPResponse(http.StatusOK), nil
	})})

	cases := []struct {
		name string
		run  func() error
	}{
		{"empty batch", func() error { return client.SetConfigFields(context.Background(), "ws-1", ConfigScopeGlobal, nil) }},
		{"blank batch key", func() error {
			return client.SetConfigFields(context.Background(), "ws-1", ConfigScopeGlobal, map[string]any{" ": true})
		}},
		{"missing model provider", func() error {
			return client.SetPreferredModelPair(context.Background(), "ws-1", ConfigScopeGlobal, SelectedModel{Model: "claude"})
		}},
		{"missing workspace", func() error {
			return client.RemovePreferredModelPair(context.Background(), "", ConfigScopeGlobal)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); err == nil {
				t.Fatal("mutation succeeded")
			}
		})
	}
	if requests != 0 {
		t.Fatalf("invalid mutations sent %d requests", requests)
	}
}
