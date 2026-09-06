package contextseed

import (
	"bytes"
	"context"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSnapshotIdentityKeyConcurrentPublishers(t *testing.T) {
	previous := runtime.GOMAXPROCS(8)
	defer runtime.GOMAXPROCS(previous)
	path := filepath.Join(t.TempDir(), ".identity-key")
	const publishers = 64
	keys := make([][]byte, publishers)
	errs := make([]error, publishers)
	start := make(chan struct{})
	var done sync.WaitGroup
	for i := range keys {
		done.Add(1)
		go func(i int) {
			defer done.Done()
			<-start
			keys[i], errs[i] = loadOrCreateSnapshotIdentityKey(path)
		}(i)
	}
	close(start)
	done.Wait()
	stored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	winner, err := parseSnapshotIdentityKey(stored)
	if err != nil {
		t.Fatal("published key is invalid")
	}
	for i, key := range keys {
		if errs[i] != nil {
			t.Errorf("publisher %d could not load the committed key", i)
		} else if !bytes.Equal(key, winner) {
			t.Errorf("publisher %d returned an identity different from the committed key", i)
		}
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != ".identity-key" {
		t.Fatal("publisher left staging files behind")
	}
}

func TestSnapshotIdentityKeyReuseAndCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".identity-key")
	first, err := loadOrCreateSnapshotIdentityKey(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := loadOrCreateSnapshotIdentityKey(path)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatal("restart changed the committed identity")
	}
	corrupt := []byte("invalid-key")
	if err := os.WriteFile(path, corrupt, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadOrCreateSnapshotIdentityKey(path); err == nil {
		t.Fatal("corrupt identity key was accepted")
	}
	stored, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(stored, corrupt) {
		t.Fatal("corrupt key was silently replaced")
	}
}

func TestSnapshotIdentityKeyReadErrorFailsClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".identity-key")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := loadOrCreateSnapshotIdentityKey(path); err == nil {
		t.Fatal("unreadable key path was accepted")
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		t.Fatal("existing key path was replaced after a read error")
	}
}

// Separate processes do not share Seeder.mu (or a process-global mutex).
func TestSnapshotIdentityKeyAcrossProcesses(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dir := t.TempDir()
	const publishers = 8
	commands := make([]*exec.Cmd, publishers)
	for i := range commands {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSnapshotIdentityKeyProcess$")
		cmd.Env = append(os.Environ(), "GOTACK_SNAPSHOT_KEY_TEST_DIR="+dir,
			"GOTACK_SNAPSHOT_KEY_TEST_RESULT="+strconv.Itoa(i))
		commands[i] = cmd
		if err := cmd.Start(); err != nil {
			t.Fatal("start isolated key publisher")
		}
		t.Cleanup(func() {
			if cmd.ProcessState == nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}
		})
	}
	for i, cmd := range commands {
		if err := cmd.Wait(); err != nil {
			t.Errorf("isolated publisher %d failed", i)
		}
	}
	stored, err := os.ReadFile(filepath.Join(dir, ".identity-key"))
	if err != nil {
		t.Fatal(err)
	}
	winner, err := parseSnapshotIdentityKey(stored)
	if err != nil {
		t.Fatal("committed process key is invalid")
	}
	for i := range commands {
		data, err := os.ReadFile(filepath.Join(dir, "result-"+strconv.Itoa(i)))
		if err != nil {
			t.Errorf("publisher %d result missing", i)
			continue
		}
		key, err := hex.DecodeString(string(data))
		if err != nil || !bytes.Equal(key, winner) {
			t.Errorf("publisher %d returned an inconsistent identity", i)
		}
	}
}

func TestSnapshotIdentityKeyProcess(t *testing.T) {
	dir := os.Getenv("GOTACK_SNAPSHOT_KEY_TEST_DIR")
	if dir == "" {
		return
	}
	result := os.Getenv("GOTACK_SNAPSHOT_KEY_TEST_RESULT")
	if _, err := strconv.Atoi(result); err != nil || strings.ContainsAny(result, `/\\`) {
		t.Fatal("invalid isolated result identifier")
	}
	key, err := loadOrCreateSnapshotIdentityKey(filepath.Join(dir, ".identity-key"))
	if err != nil {
		t.Fatal("isolated key publication failed")
	}
	if err := os.WriteFile(filepath.Join(dir, "result-"+result), []byte(hex.EncodeToString(key)), 0o600); err != nil {
		t.Fatal("write isolated result")
	}
}
