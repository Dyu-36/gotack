//go:build windows

package contextseed

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSnapshotLeaseWindowsProfileCaseAlias(t *testing.T) {
	s := New(t.TempDir(), nil)
	seedContextWithFile(t, s, "TACK_CORE.md", "one")
	gen, err := s.BuildPromptSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(strings.ToUpper(filepath.Dir(gen)), filepath.Base(gen))
	holder := New(strings.ToUpper(s.dataDir), nil)
	lease, err := holder.AcquireSnapshotLease(gen)
	if err != nil {
		t.Fatalf("profile alias rejected same generation: %v", err)
	}
	defer lease.Release()
	lease2, err := s.AcquireSnapshotLease(alias)
	if err != nil {
		t.Fatalf("generation alias rejected same generation: %v", err)
	}
	defer lease2.Release()
	seedContextWithFile(t, s, "TACK_CORE.md", "two")
	latest, err := s.BuildPromptSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := New(s.dataDir, nil).PrunePromptSnapshotsChecked(latest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(gen); err != nil {
		t.Fatalf("aliased holder lost protection: %v", err)
	}
}

func TestSnapshotAcquirePruneAcrossProcesses(t *testing.T) {
	data := t.TempDir()
	s := New(data, nil)
	seedContextWithFile(t, s, "TACK_CORE.md", "atomic lease bytes")
	ready := filepath.Join(t.TempDir(), "ready")
	cmd := exec.Command(os.Args[0], "-test.run=^TestSnapshotAcquirePruneRaceHelper$")
	cmd.Env = append(os.Environ(), "GOTACK_WP3_RACE_DATA="+data, "GOTACK_WP3_RACE_READY="+ready)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := false
	defer func() {
		if !done {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("child readiness timeout")
		}
		time.Sleep(time.Millisecond)
	}
	for i := 0; i < 80; i++ {
		if err := New(data, nil).PrunePromptSnapshotsChecked(""); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
	}
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	select {
	case err := <-wait:
		done = true
		if err != nil {
			t.Fatalf("atomic acquire/read child failed: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("child completion timeout")
	}
	if err := New(data, nil).PrunePromptSnapshotsChecked(""); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(s.PromptContextRoot())
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), snapshotPrefix) {
			t.Fatal("unleased generation leaked after race")
		}
	}
}

func TestSnapshotAcquirePruneRaceHelper(t *testing.T) {
	data := os.Getenv("GOTACK_WP3_RACE_DATA")
	if data == "" {
		return
	}
	s := New(data, nil)
	for i := 0; i < 80; i++ {
		gen, lease, err := s.BuildPromptSnapshotLease()
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			if err := os.WriteFile(os.Getenv("GOTACK_WP3_RACE_READY"), []byte("ok"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		time.Sleep(time.Millisecond)
		b, err := os.ReadFile(filepath.Join(gen, "TACK_CORE.md"))
		if err != nil || string(b) != "atomic lease bytes" {
			_ = lease.Release()
			t.Fatalf("leased bytes lost during concurrent prune: %v", err)
		}
		if err := lease.Release(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSnapshotBoundedRetentionAfterManyGenerations(t *testing.T) {
	s := New(t.TempDir(), nil)
	seedContextWithFile(t, s, "TACK_CORE.md", "old holder")
	gen1, held, err := s.BuildPromptSnapshotLease()
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	for i := 2; i <= 12; i++ {
		seedContextWithFile(t, s, "TACK_CORE.md", fmt.Sprintf("generation %d", i))
		gen, lease, err := s.BuildPromptSnapshotLease()
		if err != nil {
			t.Fatal(err)
		}
		if err := lease.Release(); err != nil {
			t.Fatal(err)
		}
		if err := s.PrunePromptSnapshotsChecked(gen); err != nil {
			t.Fatal(err)
		}
		entries, err := os.ReadDir(s.PromptContextRoot())
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), snapshotPrefix) {
				count++
			}
		}
		if count > 3 {
			t.Fatalf("retention exceeded holder/current/previous floor: %d", count)
		}
		locks, err := os.ReadDir(s.snapshotLeaseDir())
		if err != nil {
			t.Fatal(err)
		}
		if len(locks) > 4 {
			t.Fatalf("generation lease metadata leaked: %d", len(locks))
		}
		if _, err := os.Stat(gen1); err != nil {
			t.Fatalf("old holder pruned: %v", err)
		}
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	if err := New(s.dataDir, nil).PrunePromptSnapshotsChecked(""); err != nil {
		t.Fatal(err)
	}
	locks, err := os.ReadDir(s.snapshotLeaseDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(locks) != 1 || locks[0].Name() != snapshotRegistryLockName {
		t.Fatalf("released lease metadata not reclaimed: %d", len(locks))
	}
}
