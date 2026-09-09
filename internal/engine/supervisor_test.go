package engine

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/engineapi"
)

func TestMain(m *testing.M) {
	if statePath := os.Getenv("TACK_ENGINE_HELPER_STATE"); statePath != "" {
		args, err := json.Marshal(os.Args[1:])
		if err != nil || os.WriteFile(statePath, args, 0o600) != nil {
			os.Exit(1)
		}
		for {
			if _, err := os.Stat(statePath + ".exit"); err == nil {
				os.Exit(0)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	os.Exit(m.Run())
}

func newProcessSupervisor(t *testing.T) (*Supervisor, string) {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"APPDATA", "LOCALAPPDATA", "XDG_CONFIG_HOME", "XDG_CACHE_HOME"} {
		t.Setenv(name, root)
	}
	statePath := filepath.Join(root, "engine-args.json")
	t.Setenv("TACK_ENGINE_HELPER_STATE", statePath)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	sup := NewSupervisor(nil, binary)
	t.Cleanup(func() {
		if err := sup.Stop(); err != nil {
			t.Errorf("Stop() = %v", err)
		}
	})
	return sup, statePath
}

func waitForProcess(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("engine process did not exit")
	}
}

func TestSupervisorStartUsesEndpointAndReusesProcess(t *testing.T) {
	sup, statePath := newProcessSupervisor(t)
	ep, err := sup.Start()
	if err != nil {
		t.Fatal(err)
	}
	if ep != appconfig.PipeEndpoint() {
		t.Fatalf("Start() endpoint = %v, want %v", ep, appconfig.PipeEndpoint())
	}
	sup.mu.Lock()
	cmd := sup.cmd
	sup.mu.Unlock()

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			reused, err := sup.Start()
			if err != nil || reused != ep {
				t.Errorf("repeated Start() = %v, %v; want %v, nil", reused, err, ep)
			}
			sup.mu.Lock()
			defer sup.mu.Unlock()
			if sup.cmd != cmd {
				t.Error("repeated Start() launched another process")
			}
		})
	}
	wg.Wait()

	want := []string{"server", "--host", ep.Network + "://" + ep.Address}
	deadline := time.Now().Add(5 * time.Second)
	for {
		data, err := os.ReadFile(statePath)
		var args []string
		if err == nil && json.Unmarshal(data, &args) == nil {
			if !slices.Equal(args, want) {
				t.Fatalf("engine arguments = %q, want %q", args, want)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("engine helper did not report its arguments")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !sup.Owned() {
		t.Fatal("started engine is not owned")
	}
}

func TestSupervisorRestartsAfterExitAndStop(t *testing.T) {
	sup, statePath := newProcessSupervisor(t)
	if _, err := sup.Start(); err != nil {
		t.Fatal(err)
	}
	sup.mu.Lock()
	previous := sup.cmd
	done := sup.done
	sup.mu.Unlock()
	if err := os.WriteFile(statePath+".exit", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	waitForProcess(t, done)
	if sup.Owned() {
		t.Fatal("exited engine is still marked owned")
	}
	if err := os.Remove(statePath + ".exit"); err != nil {
		t.Fatal(err)
	}

	for range 2 {
		if _, err := sup.Start(); err != nil {
			t.Fatalf("restart failed: %v", err)
		}
		sup.mu.Lock()
		current := sup.cmd
		done = sup.done
		sup.mu.Unlock()
		if current == previous || current == nil {
			t.Fatal("restart did not launch a new process")
		}
		if err := sup.Stop(); err != nil {
			t.Fatal(err)
		}
		waitForProcess(t, done)
		if sup.Owned() {
			t.Fatal("stopped engine is still marked owned")
		}
		previous = current
	}
}

type unlocatedSupervisor struct {
	*Supervisor
}

func (s unlocatedSupervisor) Locate(context.Context) (engineapi.Endpoint, bool) {
	return engineapi.Endpoint{}, false
}

func TestSupervisorReconnectAfterHandshakeTimeout(t *testing.T) {
	sup, _ := newProcessSupervisor(t)
	link := NewLink(unlocatedSupervisor{sup})
	link.handshakeTimeout = 20 * time.Millisecond
	defer link.CancelScope()
	healthy := false
	transport := (&engineTransport{version: "test"}).roundTrip()
	link.dial = func(engineapi.Endpoint) (*http.Client, error) {
		return &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if !healthy {
				return nil, errors.New("engine is still starting")
			}
			return transport.RoundTrip(req)
		})}, nil
	}
	scope, _ := link.BeginConnect(context.Background())
	ready := func(ctx context.Context, _ *engineapi.Client, ep engineapi.Endpoint, version string) error {
		if !link.CommitAttach(ctx, ep, version) {
			return ErrAttachSuperseded
		}
		link.MarkRunning()
		return nil
	}
	err := link.Connect(scope, ready)
	if !errors.Is(err, ErrEngineUnhealthy) {
		t.Fatalf("initial Connect() = %v, want handshake timeout", err)
	}
	link.Fail(err.Error())
	sup.mu.Lock()
	cmd := sup.cmd
	sup.mu.Unlock()
	healthy = true
	scope, started := link.BeginConnect(context.Background())
	if !started {
		t.Fatal("retry was rejected")
	}
	if err := link.Connect(scope, ready); err != nil {
		t.Fatalf("reconnect failed: %v", err)
	}
	if link.Status() != StatusRunning {
		t.Fatalf("status = %v, want running", link.Status())
	}
	sup.mu.Lock()
	defer sup.mu.Unlock()
	if sup.cmd != cmd {
		t.Fatal("reconnect replaced the engine that was still starting")
	}
}
