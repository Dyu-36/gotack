package zalo

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestUnpairCancelsOnlyRevokedChat(t *testing.T) {
	var cancelled []string
	m := NewManager(filepath.Join(t.TempDir(), "zalo.json"), Runtime{Stop: func(_ context.Context, run Turn) error {
		cancelled = append(cancelled, run.SessionID)
		return nil
	}}, nil)
	m.state.PairedChatIDs = []string{"a", "b"}
	m.state.ChatSessions = map[string]string{"a": "one", "b": "two"}
	m.active = map[string]*activeTurn{"a": testActiveTurn("one"), "b": testActiveTurn("two")}
	if _, err := m.Unpair("a"); err != nil {
		t.Fatal(err)
	}
	if len(cancelled) != 1 || cancelled[0] != "one" {
		t.Fatalf("cancelled=%v", cancelled)
	}
	if m.paired("a") || !m.paired("b") || m.active["b"].SessionID != "two" {
		t.Fatal("incorrect revocation scope")
	}
	reloaded := NewManager(m.path, Runtime{}, nil)
	if reloaded.paired("a") {
		t.Fatal("revocation was not persisted")
	}
}

func TestStopCancelsRemoteRunsWithoutDuplicateCancellation(t *testing.T) {
	var cancelled []string
	m := NewManager(filepath.Join(t.TempDir(), "zalo.json"), Runtime{Stop: func(_ context.Context, run Turn) error {
		cancelled = append(cancelled, run.SessionID)
		return nil
	}}, nil)
	m.active = map[string]*activeTurn{"a": testActiveTurn("one"), "b": testActiveTurn("one"), "c": testActiveTurn("")}
	m.Stop()
	m.Stop()
	if len(cancelled) != 1 || cancelled[0] != "one" {
		t.Fatalf("cancelled=%v", cancelled)
	}
	if len(m.active) != 0 {
		t.Fatal("stopped channel retains active work")
	}
}

func TestCorruptSavedChannelDoesNotPartiallyGrantAccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "zalo.json")
	if err := os.WriteFile(path, []byte(`{"token":"secret","paired_chat_ids":["intruder"],"update_offset":"invalid"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	m := NewManager(path, Runtime{}, nil)
	if m.Status().Configured || m.paired("intruder") {
		t.Fatal("partially decoded corrupt state was trusted")
	}
}

func TestRuntimeSnapshotIsSafeDuringReplacement(t *testing.T) {
	m := NewManager(filepath.Join(t.TempDir(), "zalo.json"), Runtime{}, nil)
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.SetRuntime(Runtime{Workspace: func() string { return "workspace" }})
			if runtime := m.runtimeSnapshot(); runtime.Workspace != nil {
				_ = runtime.Workspace()
			}
		}()
	}
	wg.Wait()
}

func testActiveTurn(sessionID string) *activeTurn {
	ctx, cancel := context.WithCancel(context.Background())
	return &activeTurn{Turn: Turn{ID: "run-" + sessionID, WorkspaceID: "workspace", SessionID: sessionID}, ctx: ctx, cancel: cancel}
}
