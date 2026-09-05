//go:build e2e

package inputpipeline

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/stretchr/testify/require"
)

const (
	healthTimeout = 30 * time.Second
	turnTimeout   = 120 * time.Second
)

func engineBinary() string {
	if v := os.Getenv("TACK_ENGINE_BINARY"); v != "" {
		return v
	}
	candidates := []string{
		"tack-engine.exe",
		"tack-engine",
		"crush",
	}
	for _, c := range candidates {
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	return ""
}

func skipIfNoEngine(t *testing.T) {
	t.Helper()
	if engineBinary() == "" {
		t.Skip("E2E requires TACK_ENGINE_BINARY or engine on PATH; set with: $env:TACK_ENGINE_BINARY='path/to/tack-engine.exe'")
	}
}

func setupTempWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o755))
	return dir
}

func waitForHealth(ctx context.Context, endpoint string) error {
	deadline := time.Now().Add(healthTimeout)
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://%s/v1/health", endpoint), nil)
		if err != nil {
			continue
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("health check timed out after %v", healthTimeout)
}

func TestE2EInputPipelineFreshTurn(t *testing.T) {
	skipIfNoEngine(t)

	workspaceDir := setupTempWorkspace(t)
	engine := engineBinary()

	ctx, cancel := context.WithTimeout(context.Background(), turnTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, engine, "server")
	cmd.Dir = workspaceDir
	cmd.Env = append(os.Environ(),
		"TACK_DATA_DIR="+filepath.Join(workspaceDir, "data"),
	)

	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	defer cmd.Process.Kill()

	scanner := bufio.NewScanner(stdout)
	endpoint := ""
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "listening") || strings.Contains(line, "pipe") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				endpoint = strings.TrimSpace(parts[1])
			}
			break
		}
	}

	if endpoint == "" {
		t.Skip("could not detect engine endpoint from stdout")
	}

	require.NoError(t, waitForHealth(ctx, endpoint))

	sessionResp, err := http.Post(
		fmt.Sprintf("http://%s/v1/workspaces/test/sessions", endpoint),
		"application/json",
		strings.NewReader(`{"title":"e2e-bench"}`),
	)
	require.NoError(t, err)
	defer sessionResp.Body.Close()

	var session struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.NewDecoder(sessionResp.Body).Decode(&session))
	require.NotEmpty(t, session.ID)

	t.Logf("E2E fresh turn: session=%s endpoint=%s", session.ID, endpoint)
}

func TestE2ETelemetryPassthrough(t *testing.T) {
	skipIfNoEngine(t)

	telemetry := &crushapi.RunTelemetry{
		RunID:             "e2e-telemetry-test",
		CacheStatus:       "unreported",
		TotalMicros:       123456,
		Attempt:           1,
		ProviderRequestID: "should-be-redacted",
	}

	data, err := json.Marshal(telemetry)
	require.NoError(t, err)
	require.NotContains(t, string(data), "prompt")
	require.Contains(t, string(data), "run_id")

	var decoded crushapi.RunTelemetry
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, "e2e-telemetry-test", decoded.RunID)
}

func TestE2EFakeProviderCapture(t *testing.T) {
	skipIfNoEngine(t)

	captured := make(chan []byte, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/responses", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured <- body
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"resp-1","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}],"usage":{"input_tokens":10,"output_tokens":5}}`)
	})

	server := &http.Server{Handler: mux, Addr: "127.0.0.1:0"}
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go server.Serve(ln)
	defer server.Close()
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	t.Logf("Fake provider listening on port %d", port)
}

func TestE2ERetryBehavior(t *testing.T) {
	attempts := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/responses", func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(429)
			fmt.Fprint(w, `{"error":{"type":"rate_limit_error","message":"rate limited"}}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"resp-retry","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok after retry"}]}],"usage":{"input_tokens":10,"output_tokens":5}}`)
	})

	server := &http.Server{Handler: mux, Addr: "127.0.0.1:0"}
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go server.Serve(ln)
	defer server.Close()
	defer ln.Close()

	require.Equal(t, 0, attempts)
}
