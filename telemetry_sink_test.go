package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/runmetrics"
	"github.com/Dyu-36/gotack/internal/uievents"
)

func TestWorkspaceStreamCannotFabricateProviderSpanInHostSink(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, ": workspace heartbeat\n\n")
		w.(http.Flusher).Flush()
		for i := 1; i <= 2; i++ {
			time.Sleep(10 * time.Millisecond)
			fmt.Fprintf(w, "data: {\"type\":\"run_complete\",\"payload\":{\"session_id\":\"private-session\",\"run_id\":\"run-%d\",\"telemetry\":{\"run_id\":\"run-%d\",\"cache_status\":\"unreported\",\"total_us\":100,\"spans_us\":{\"stream\":42}}}}\n\n", i, i)
			w.(http.Flusher).Flush()
		}
	}))
	defer srv.Close()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	defer transport.CloseIdleConnections()
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
	}
	api := crushapi.NewClient(&http.Client{Transport: transport})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	events, stop, err := api.Stream(ctx, "fixture", "run_complete")
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	dir := t.TempDir()
	a := &App{runMetrics: runmetrics.New(dir, nil)}
	fwd := uievents.NewForwarder(nil, func(string, any) {}, uievents.Callbacks{RunTelemetry: a.telemetryCallback(api)})
	defer fwd.Stop()
	fwd.Consume(events)
	data, err := os.ReadFile(filepath.Join(dir, "input-pipeline.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("sink records=%d, want 2", len(lines))
	}
	for _, line := range lines {
		var record crushapi.RunTelemetry
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal(err)
		}
		if _, ok := record.SpansMicros["first_byte_to_first_sse"]; ok {
			t.Fatal("workspace SSE fabricated a provider span in host JSONL sink")
		}
		if record.SpansMicros["stream"] != 42 {
			t.Fatal("engine span lost")
		}
	}
	if strings.Contains(string(data), "private-session") {
		t.Fatal("session canary leaked")
	}
}
