package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Dyu-36/gotack/internal/engine"
	"github.com/Dyu-36/gotack/internal/engineapi"
	"github.com/Dyu-36/gotack/internal/modelcatalog"
	"github.com/Dyu-36/gotack/internal/provider"
	"github.com/google/uuid"
)

type bridgePiCatalogSource struct {
	mu      sync.RWMutex
	catalog modelcatalog.Catalog
}

func (s *bridgePiCatalogSource) Load(ctx context.Context) (modelcatalog.Catalog, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.Marshal(s.catalog)
	if err != nil {
		return nil, err
	}
	var catalog modelcatalog.Catalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, err
	}
	return catalog, nil
}

func (s *bridgePiCatalogSource) replace(catalog modelcatalog.Catalog) {
	s.mu.Lock()
	s.catalog = catalog
	s.mu.Unlock()
}

func TestBridgePiCatalogModels(t *testing.T) {
	api, root := bridgePiCatalogTestRuntime(t)
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()

	var requestMu sync.Mutex
	var requestedModels []string
	providerHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/models" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"object":"list","data":[{"id":"pi-contract-v1","object":"model"},{"id":"pi-contract-v2","object":"model"}]}`)
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			http.Error(w, "unexpected provider endpoint", http.StatusNotFound)
			return
		}
		var request struct {
			Model  string `json:"model"`
			Stream bool   `json:"stream"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		requestMu.Lock()
		requestedModels = append(requestedModels, request.Model)
		requestMu.Unlock()

		const responseText = "Pi catalog contract verified"
		if !request.Stream {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "chatcmpl-pi-contract", "object": "chat.completion", "created": time.Now().Unix(), "model": request.Model,
				"choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": responseText}, "finish_reason": "stop"}},
				"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
			})
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		writeChunk := func(delta map[string]any, finish any) {
			body, _ := json.Marshal(map[string]any{
				"id": "chatcmpl-pi-contract", "object": "chat.completion.chunk", "created": time.Now().Unix(), "model": request.Model,
				"choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}},
			})
			_, _ = fmt.Fprintf(w, "data: %s\n\n", body)
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		writeChunk(map[string]any{"role": "assistant", "content": responseText}, nil)
		writeChunk(map[string]any{}, "stop")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer providerHTTP.Close()

	ws, err := api.CreateWorkspace(ctx, root, true)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	source := &bridgePiCatalogSource{catalog: bridgePiCatalog(providerHTTP.URL, false)}
	catalog := provider.NewPiCatalog(source, filepath.Join(t.TempDir(), "pi-manifest.json"))
	if err := api.SetProviderAPIKey(ctx, ws.ID, engineapi.ConfigScopeGlobal, "openai", "contract-key"); err != nil {
		t.Fatalf("set known provider key: %v", err)
	}
	const batchModelID = "deepseek/deepseek-v4.1-flash:batch"
	if err := api.SetConfigFields(ctx, ws.ID, engineapi.ConfigScopeWorkspace, map[string]any{
		"providers.openrouter.api_key":         "contract-key",
		"providers.openrouter.base_url":        providerHTTP.URL + "/v1",
		"providers.openrouter.catalog_models":  true,
		"providers.openrouter.discover_models": false,
		"providers.openrouter.models":          []provider.LocalEngineModel{{ID: batchModelID, Name: "Saved batch model"}},
		"providers.openrouter.model_routes":    map[string]engineapi.ModelRoute{batchModelID: {API: "openai-completions"}},
	}); err != nil {
		t.Fatalf("seed saved OpenRouter batch model: %v", err)
	}
	if err := api.SetPreferredModelPair(ctx, ws.ID, engineapi.ConfigScopeWorkspace, engineapi.SelectedModel{Provider: "openrouter", Model: batchModelID}); err != nil {
		t.Fatalf("seed saved batch selection: %v", err)
	}

	listed, err := catalog.List(ctx, api, ws.ID)
	if err != nil {
		t.Fatalf("list Pi catalog providers: %v", err)
	}
	if !bridgeHasPiModel(listed, "pi-contract", "pi-contract-v1") {
		t.Fatalf("first Pi model is not selectable from provider catalog: %+v", listed)
	}
	if bridgeHasPiModel(listed, "openrouter", batchModelID) || !bridgeHasPiModel(listed, "openrouter", "deepseek/deepseek-v4.1-flash") {
		t.Fatal("OpenRouter picker must offer the base chat model and exclude the saved batch variant")
	}
	batchConfig, err := api.GetWorkspaceConfig(ctx, ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	if bridgeHasConfiguredPiModel(batchConfig.Providers["openrouter"].Models, batchModelID) {
		t.Fatal("saved batch model was not removed from real engine config")
	}
	if _, exists := batchConfig.Providers["openrouter"].ModelRoutes[batchModelID]; exists {
		t.Fatal("saved batch route was not removed from real engine config")
	}
	knownConfigured := false
	for _, entry := range listed {
		if entry.ID == "openai" {
			knownConfigured = entry.Configured && entry.CredentialKind == "api_key"
		}
	}
	if !knownConfigured {
		t.Fatal("known engine provider credentials were not projected into the Pi catalog")
	}
	if err := catalog.Prepare(ctx, api, ws.ID, "pi-contract"); err != nil {
		t.Fatalf("prepare Pi provider: %v", err)
	}
	if err := api.SetProviderAPIKey(ctx, ws.ID, engineapi.ConfigScopeWorkspace, "pi-contract", "contract-key"); err != nil {
		t.Fatalf("set Pi provider key: %v", err)
	}
	if err := api.SetPreferredModelPair(ctx, ws.ID, engineapi.ConfigScopeWorkspace, engineapi.SelectedModel{Provider: "pi-contract", Model: "pi-contract-v1"}); err != nil {
		t.Fatalf("select first Pi model: %v", err)
	}
	if err := api.InitAgent(ctx, ws.ID, false); err != nil {
		t.Fatalf("initialize real agent: %v", err)
	}

	// Refresh the source after agent initialization, then prepare and select its
	// newly published model without restarting the running agent.
	source.replace(bridgePiCatalog(providerHTTP.URL, true))
	refreshed, err := catalog.List(ctx, api, ws.ID)
	if err != nil {
		t.Fatalf("refresh Pi catalog providers: %v", err)
	}
	if !bridgeHasPiModel(refreshed, "pi-contract", "pi-contract-v2") {
		t.Fatalf("refreshed Pi model is not selectable from provider catalog: %+v", refreshed)
	}
	if err := catalog.Prepare(ctx, api, ws.ID, "pi-contract"); err != nil {
		t.Fatalf("prepare refreshed Pi provider: %v", err)
	}
	if err := api.SetPreferredModelPair(ctx, ws.ID, engineapi.ConfigScopeWorkspace, engineapi.SelectedModel{Provider: "pi-contract", Model: "pi-contract-v2"}); err != nil {
		t.Fatalf("select refreshed Pi model: %v", err)
	}
	config, err := api.GetWorkspaceConfig(ctx, ws.ID)
	if err != nil {
		t.Fatalf("read persisted Pi provider config: %v", err)
	}
	configured, exists := config.Providers["pi-contract"]
	if !exists || configured.BaseURL != providerHTTP.URL+"/v1" || !bridgeHasConfiguredPiModel(configured.Models, "pi-contract-v2") {
		t.Fatalf("refreshed model was not persisted for engine selection: %+v", configured)
	}
	if !configured.CatalogModels || configured.ModelRoutes["pi-contract-v2"].API != "openai-completions" {
		t.Fatalf("engine lost the authoritative Pi model routing metadata: %+v", configured)
	}

	session, err := api.CreateSession(ctx, ws.ID, "pi-catalog-contract")
	if err != nil {
		t.Fatal(err)
	}
	stream, stop, err := api.Stream(ctx, ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	if err := api.SetCurrentSession(ctx, ws.ID, session.ID); err != nil {
		t.Fatal(err)
	}
	runID := uuid.NewString()
	if err := api.SendPromptWithPurpose(ctx, ws.ID, session.ID, "Reply with the catalog contract result.", runID, "pi-catalog-test", nil); err != nil {
		t.Fatalf("send prompt with refreshed Pi model selected: %v", err)
	}
	var completion engineapi.RunComplete
wait:
	for {
		select {
		case <-ctx.Done():
			requestMu.Lock()
			got := append([]string(nil), requestedModels...)
			requestMu.Unlock()
			t.Fatalf("real agent did not complete with refreshed Pi model; provider requests used %v", got)
		case event, ok := <-stream:
			if !ok {
				t.Fatal("engine event stream ended before Pi catalog run completed")
			}
			if event.Kind != "run_complete" {
				continue
			}
			if err := json.Unmarshal(event.Payload, &completion); err != nil {
				t.Fatal(err)
			}
			if completion.RunID == runID {
				break wait
			}
		}
	}
	if completion.SessionID != session.ID || completion.Error != "" || completion.Cancelled || !strings.Contains(completion.Text, "Pi catalog contract verified") {
		t.Fatalf("unexpected real-agent terminal event: %+v", completion)
	}
	requestMu.Lock()
	got := append([]string(nil), requestedModels...)
	requestMu.Unlock()
	if len(got) == 0 || got[len(got)-1] != "pi-contract-v2" {
		t.Fatalf("provider did not receive the refreshed catalog model ID: got %v", got)
	}

	// Catalog seeding must not shadow a later endpoint change saved by settings.
	customURL := providerHTTP.URL + "/custom/v1"
	if err := provider.Apply(ctx, api, ws.ID, provider.Settings{
		CredentialProvider: "pi-contract", ProviderOnly: true,
		CustomURL: customURL, CatalogManaged: true,
	}, ""); err != nil {
		t.Fatalf("save custom Pi provider endpoint: %v", err)
	}
	if _, err := catalog.List(ctx, api, ws.ID); err != nil {
		t.Fatalf("refresh Pi catalog after endpoint edit: %v", err)
	}
	config, err = api.GetWorkspaceConfig(ctx, ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	configured = config.Providers["pi-contract"]
	if configured.BaseURL != customURL || configured.ModelRoutes["pi-contract-v2"].BaseURL != "" {
		t.Fatalf("Pi catalog overwrote the custom provider endpoint: %+v", configured)
	}
}

func bridgePiCatalogTestRuntime(t *testing.T) (*engineapi.Client, string) {
	t.Helper()
	binary := os.Getenv("GOTACK_TEST_ENGINE")
	if binary == "" {
		if os.Getenv("GOTACK_REQUIRE_ENGINE") == "1" {
			t.Fatal("GOTACK_TEST_ENGINE must identify the built pinned engine")
		}
		t.Skip("set GOTACK_TEST_ENGINE to run the real host/engine contract tests")
	}
	absolute, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(absolute); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("engine executable is unavailable: %s (%v)", absolute, err)
	}
	root := t.TempDir()
	for _, variable := range []string{"APPDATA", "LOCALAPPDATA", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_RUNTIME_DIR"} {
		t.Setenv(variable, root)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	endpoint := engineapi.Endpoint{Network: "unix", Address: filepath.Join(root, "gotack-pi-contract-"+uuid.NewString()+".sock")}
	if runtime.GOOS == "windows" {
		endpoint = engineapi.Endpoint{Network: "npipe", Address: `\\.\pipe\gotack-pi-contract-` + uuid.NewString()}
	}
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	sup := engine.NewSupervisorWithEndpoint(slog.Default(), absolute, endpoint)
	if _, exists := sup.Locate(ctx); exists {
		t.Fatal("the unique test IPC endpoint is already occupied; refusing to use another engine")
	}
	ep, err := sup.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sup.Stop(); err != nil {
			t.Errorf("stop test engine: %v", err)
		}
	})
	hc, err := engineapi.Dial(ep)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(hc.CloseIdleConnections)
	api := engineapi.NewClient(hc)
	vi, err := engine.WaitForHealthy(ctx, api, 45*time.Second)
	if err != nil {
		t.Fatalf("engine handshake: %v", err)
	}
	if packagedEngineCommit != "" && vi.Commit != strings.TrimSpace(packagedEngineCommit) {
		t.Fatalf("wrong engine: expected %s, got %s", strings.TrimSpace(packagedEngineCommit), vi.Commit)
	}
	t.Logf("verified engine commit=%s version=%s platform=%s", vi.Commit, vi.Version, vi.Platform)
	workspaceRoot := filepath.Join(root, "workspace")
	if err := os.MkdirAll(workspaceRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	return api, workspaceRoot
}

func bridgePiCatalog(baseURL string, includeSecond bool) modelcatalog.Catalog {
	models := map[string]modelcatalog.Model{
		"pi-contract-v1": {
			ID: "pi-contract-v1", Name: "Pi contract v1", API: "openai-completions", Provider: "pi-contract",
			BaseURL: baseURL + "/v1", Input: []string{"text"}, ContextWindow: 32768, MaxTokens: 2048, Type: "chat",
		},
	}
	if includeSecond {
		models["pi-contract-v2"] = modelcatalog.Model{
			ID: "pi-contract-v2", Name: "Pi contract v2", API: "openai-completions", Provider: "pi-contract",
			BaseURL: baseURL + "/v1", Input: []string{"text"}, ContextWindow: 65536, MaxTokens: 4096, Type: "chat",
		}
	}
	return modelcatalog.Catalog{
		"openrouter": {
			"deepseek/deepseek-v4.1-flash":       {ID: "deepseek/deepseek-v4.1-flash", API: "openai-completions", BaseURL: baseURL + "/v1", Input: []string{"text"}, Type: "chat"},
			"deepseek/deepseek-v4.1-flash:batch": {ID: "deepseek/deepseek-v4.1-flash:batch", API: "openai-completions", BaseURL: baseURL + "/v1", Input: []string{"text"}, Type: "chat"},
		},
		"pi-contract": models,
		"openai": {
			"pi-openai-contract": {
				ID: "pi-openai-contract", Name: "Pi OpenAI contract", API: "openai-completions", Provider: "openai",
				BaseURL: baseURL + "/v1", Input: []string{"text"}, ContextWindow: 32768, MaxTokens: 2048, Type: "chat",
			},
		},
	}
}

func bridgeHasPiModel(providers []engineapi.Provider, providerID, modelID string) bool {
	for _, candidate := range providers {
		if candidate.ID != providerID {
			continue
		}
		return bridgeHasConfiguredPiModel(candidate.Models, modelID)
	}
	return false
}

func bridgeHasConfiguredPiModel(models []engineapi.Model, modelID string) bool {
	for _, model := range models {
		if model.ID == modelID {
			return true
		}
	}
	return false
}
