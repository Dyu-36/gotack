//go:build windows

package contextseed

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const snapshotLeaseHelperEnv = "GOTACK_SNAPSHOT_LEASE_HELPER"

func TestSnapshotLeaseSurvivesCrossProcessPruneAndCrashReleasesLock(t *testing.T) {
	dataDir := t.TempDir()
	publisher := New(dataDir, nil)
	gen1 := snapshotWithProfile(t, publisher, "gen one")
	ready := filepath.Join(t.TempDir(), "ready")
	cmd := exec.Command(os.Args[0], "-test.run=^TestSnapshotLeaseHelperProcess$")
	cmd.Env = append(os.Environ(), snapshotLeaseHelperEnv+"=1",
		"GOTACK_SNAPSHOT_LEASE_DATA="+dataDir,
		"GOTACK_SNAPSHOT_LEASE_GENERATION="+gen1,
		"GOTACK_SNAPSHOT_LEASE_READY="+ready)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	childDone := false
	defer func() {
		if !childDone && cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		data, readErr := os.ReadFile(ready)
		if readErr == nil {
			if string(data) != "ok" {
				t.Fatalf("lease helper failed: %s", data)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("lease helper did not become ready: %v", readErr)
		}
		time.Sleep(10 * time.Millisecond)
	}
	snapshotWithProfile(t, publisher, "gen two")
	snapshotWithProfile(t, publisher, "gen three")
	gen4 := snapshotWithProfile(t, publisher, "gen four")
	pruner := New(dataDir, nil)
	if err := pruner.PrunePromptSnapshotsChecked(gen4); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(gen1); err != nil {
		t.Fatalf("cross-process leased generation was pruned: %v", err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_, _ = cmd.Process.Wait()
	childDone = true
	if err := pruner.PrunePromptSnapshotsChecked(gen4); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(gen1); !os.IsNotExist(err) {
		t.Fatalf("generation should prune after crashed holder releases OS lock: %v", err)
	}
}

func TestSnapshotLeaseHelperProcess(t *testing.T) {
	if os.Getenv(snapshotLeaseHelperEnv) != "1" {
		return
	}
	dataDir := os.Getenv("GOTACK_SNAPSHOT_LEASE_DATA")
	generation := os.Getenv("GOTACK_SNAPSHOT_LEASE_GENERATION")
	ready := os.Getenv("GOTACK_SNAPSHOT_LEASE_READY")
	lease, err := New(dataDir, nil).AcquireSnapshotLease(generation)
	if err != nil {
		_ = os.WriteFile(ready, []byte(fmt.Sprintf("error:%v", err)), 0o600)
		return
	}
	defer lease.Release()
	if err := os.WriteFile(ready, []byte("ok"), 0o600); err != nil {
		return
	}
	for {
		time.Sleep(time.Second)
	}
}
