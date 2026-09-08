package engineapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type testRoundTripper func(*http.Request) (*http.Response, error)

func (f testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func testHTTPResponse(req *http.Request, status int, body ...string) *http.Response {
	payload := ""
	if len(body) > 0 {
		payload = body[0]
	}
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(payload)),
		Request:    req,
	}
}

func TestSetConfigFieldsPostsAtomicBatch(t *testing.T) {
	var got struct {
		Scope  int                        `json:"scope"`
		Fields map[string]json.RawMessage `json:"fields"`
	}
	client := NewClient(&http.Client{Transport: testRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/workspaces/ws-1/config/set-batch" {
			t.Fatalf("request = %s %s", req.Method, req.URL.Path)
		}
		if err := json.NewDecoder(req.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		return testHTTPResponse(req, http.StatusOK), nil
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
	client := NewClient(&http.Client{Transport: testRoundTripper(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Method != http.MethodPost || req.URL.Path != "/v1/workspaces/ws-1/config/models" {
			t.Fatalf("request = %s %s", req.Method, req.URL.Path)
		}
		if err := json.NewDecoder(req.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		return testHTTPResponse(req, http.StatusOK), nil
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
	client := NewClient(&http.Client{Transport: testRoundTripper(func(req *http.Request) (*http.Response, error) {
		requests++
		return testHTTPResponse(req, http.StatusOK), nil
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

func TestPreferredModelPairFallsBackToStandardEngine(t *testing.T) {
	selected := SelectedModel{Provider: "anthropic", Model: "claude", ReasoningEffort: "high", Think: true}
	var paths []string
	var got []struct {
		Scope     int           `json:"scope"`
		ModelType string        `json:"model_type"`
		Model     SelectedModel `json:"model"`
	}
	client := NewClient(&http.Client{Transport: testRoundTripper(func(req *http.Request) (*http.Response, error) {
		paths = append(paths, req.URL.Path)
		switch req.URL.Path {
		case "/v1/workspaces/ws-1/config/models":
			return testHTTPResponse(req, http.StatusNotFound, "404 page not found"), nil
		case "/v1/workspaces/ws-1/config/model":
			var payload struct {
				Scope     int           `json:"scope"`
				ModelType string        `json:"model_type"`
				Model     SelectedModel `json:"model"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode legacy model request: %v", err)
			}
			got = append(got, payload)
			return testHTTPResponse(req, http.StatusOK), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})})
	if err := client.SetPreferredModelPair(context.Background(), "ws-1", ConfigScopeGlobal, selected); err != nil {
		t.Fatalf("SetPreferredModelPair() error = %v", err)
	}
	wantPaths := []string{"/v1/workspaces/ws-1/config/models", "/v1/workspaces/ws-1/config/model", "/v1/workspaces/ws-1/config/model"}
	if fmt.Sprint(paths) != fmt.Sprint(wantPaths) {
		t.Fatalf("request paths = %#v, want %#v", paths, wantPaths)
	}
	if len(got) != 2 || got[0].ModelType != "large" || got[1].ModelType != "small" {
		t.Fatalf("legacy model payloads = %#v", got)
	}
	for _, payload := range got {
		if payload.Scope != ConfigScopeGlobal || payload.Model != selected {
			t.Fatalf("legacy model payload = %#v, want scope=%d model=%#v", payload, ConfigScopeGlobal, selected)
		}
	}
}

func TestPreferredModelPairDoesNotMaskNon404Failures(t *testing.T) {
	requests := 0
	client := NewClient(&http.Client{Transport: testRoundTripper(func(req *http.Request) (*http.Response, error) {
		requests++
		return testHTTPResponse(req, http.StatusInternalServerError, "write failed"), nil
	})})
	err := client.SetPreferredModelPair(context.Background(), "ws-1", ConfigScopeGlobal, SelectedModel{Provider: "openai", Model: "gpt"})
	if err == nil {
		t.Fatal("SetPreferredModelPair() succeeded")
	}
	if requests != 1 {
		t.Fatalf("non-404 failure sent %d requests, want 1", requests)
	}
	if !strings.Contains(err.Error(), "500 write failed") {
		t.Fatalf("error = %q", err)
	}
}

func TestRemovePreferredModelPairFallsBackToStandardConfigRemoval(t *testing.T) {
	var keys []string
	client := NewClient(&http.Client{Transport: testRoundTripper(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/workspaces/ws-1/config/models":
			return testHTTPResponse(req, http.StatusNotFound, "404 page not found"), nil
		case "/v1/workspaces/ws-1/config/remove":
			var payload struct {
				Scope int    `json:"scope"`
				Key   string `json:"key"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode legacy removal request: %v", err)
			}
			if payload.Scope != ConfigScopeWorkspace {
				t.Fatalf("removal scope = %d, want %d", payload.Scope, ConfigScopeWorkspace)
			}
			keys = append(keys, payload.Key)
			return testHTTPResponse(req, http.StatusOK), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})})
	if err := client.RemovePreferredModelPair(context.Background(), "ws-1", ConfigScopeWorkspace); err != nil {
		t.Fatalf("RemovePreferredModelPair() error = %v", err)
	}
	want := []string{"models.large", "models.small"}
	if fmt.Sprint(keys) != fmt.Sprint(want) {
		t.Fatalf("removed keys = %#v, want %#v", keys, want)
	}
}

func TestSetConfigFieldsFallsBackToStandardConfigSet(t *testing.T) {
	var writes []struct {
		Scope int             `json:"scope"`
		Key   string          `json:"key"`
		Value json.RawMessage `json:"value"`
	}
	client := NewClient(&http.Client{Transport: testRoundTripper(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/workspaces/ws-1/config/set-batch":
			return testHTTPResponse(req, http.StatusNotFound, "404 page not found"), nil
		case "/v1/workspaces/ws-1/config/set":
			var payload struct {
				Scope int             `json:"scope"`
				Key   string          `json:"key"`
				Value json.RawMessage `json:"value"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode legacy config request: %v", err)
			}
			writes = append(writes, payload)
			return testHTTPResponse(req, http.StatusOK), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})})
	err := client.SetConfigFields(context.Background(), "ws-1", ConfigScopeGlobal, map[string]any{"providers.mistral.name": "Mistral AI", "providers.mistral.discover_models": false})
	if err != nil {
		t.Fatalf("SetConfigFields() error = %v", err)
	}
	if len(writes) != 2 {
		t.Fatalf("legacy writes = %#v", writes)
	}
	if writes[0].Key != "providers.mistral.discover_models" || string(writes[0].Value) != "false" {
		t.Fatalf("first legacy write = %#v", writes[0])
	}
	if writes[1].Key != "providers.mistral.name" || string(writes[1].Value) != `"Mistral AI"` {
		t.Fatalf("second legacy write = %#v", writes[1])
	}
	for _, write := range writes {
		if write.Scope != ConfigScopeGlobal {
			t.Fatalf("legacy write scope = %d, want %d", write.Scope, ConfigScopeGlobal)
		}
	}
}

func TestIsHTTPStatusRecognizesWrappedEngineErrors(t *testing.T) {
	err := fmt.Errorf("save settings: %w", fmt.Errorf("engineapi: POST /v1/workspaces/ws-1/config/models: 404 page not found"))
	if !isHTTPStatus(err, http.StatusNotFound) {
		t.Fatalf("isHTTPStatus(%q, 404) = false", err)
	}
	if isHTTPStatus(err, http.StatusInternalServerError) {
		t.Fatalf("isHTTPStatus(%q, 500) = true", err)
	}
}
