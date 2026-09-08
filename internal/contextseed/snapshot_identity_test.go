package contextseed

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Dyu-36/gotack/internal/memory"
)

func TestSnapshotIdentityStableAcrossRefreshRestartAndMtime(t *testing.T) {
	s := New(t.TempDir(), nil)
	first := snapshotWithProfile(t, s, "same preference")
	path := filepath.Join(s.ContextDir(), memory.ProfileFileName)
	if err := os.Chtimes(path, time.Unix(10, 0), time.Unix(20, 0)); err != nil {
		t.Fatal(err)
	}
	second, err := s.BuildPromptSnapshot()
	if err != nil || first != second {
		t.Fatalf("mtime changed identity: %q %q %v", first, second, err)
	}
	restarted, err := New(s.dataDir, nil).BuildPromptSnapshot()
	if err != nil || first != restarted {
		t.Fatalf("restart changed identity: %q %q %v", first, restarted, err)
	}
}

func TestSameSizePersonalEditRotatesIdentity(t *testing.T) {
	s := New(t.TempDir(), nil)
	first := snapshotWithProfile(t, s, "Alpha")
	second := snapshotWithProfile(t, s, "Bravo")
	if first == second {
		t.Fatal("same-size content change did not rotate identity")
	}
	if string(mustRead(t, profilePath(first))) != profilePayload("Alpha") {
		t.Fatal("new publication modified a committed revision")
	}
}

func TestSnapshotIdentityUsesInstallKeyAndVersionedManifest(t *testing.T) {
	s := New(t.TempDir(), nil)
	gen := snapshotWithProfile(t, s, "same preference")
	key, err := parseSnapshotIdentityKey(mustRead(t, s.identityKeyPath()))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := s.collectSnapshot(s.ContextDir())
	if err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(snapshotIdentityDomain))
	mac.Write([]byte{0})
	mac.Write(manifest.encode())
	want := snapshotPrefix + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if filepath.Base(gen) != want {
		t.Fatal("snapshot identity is not the keyed canonical manifest")
	}
	other := New(t.TempDir(), nil)
	if otherGen := snapshotWithProfile(t, other, "same preference"); filepath.Base(otherGen) == filepath.Base(gen) {
		t.Fatal("separate installs share a user-content fingerprint")
	}
	if !strings.Contains(string(manifest.encode()), "version=2\n") {
		t.Fatal("new prompt layout did not change the identity contract")
	}
}

func TestFailedSnapshotRefreshKeepsCommittedRevision(t *testing.T) {
	s := New(t.TempDir(), nil)
	gen := snapshotWithProfile(t, s, "committed preference")
	if err := os.RemoveAll(s.ContextDir()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BuildPromptSnapshot(); err == nil {
		t.Fatal("missing source directory was accepted")
	}
	if string(mustRead(t, profilePath(gen))) != profilePayload("committed preference") {
		t.Fatal("failed source refresh changed committed bytes")
	}
}

func TestSnapshotKeyFailureHasNoUnkeyedFallback(t *testing.T) {
	s := New(t.TempDir(), nil)
	gen := snapshotWithProfile(t, s, "committed preference")
	if err := os.WriteFile(s.identityKeyPath(), []byte("not-a-key"), 0o600); err != nil {
		t.Fatal(err)
	}
	seedContextWithFile(t, s, memory.ProfileFileName, "new preference")
	if _, err := s.BuildPromptSnapshot(); err == nil {
		t.Fatal("invalid key fell back to a new identity")
	}
	if string(mustRead(t, profilePath(gen))) != profilePayload("committed preference") {
		t.Fatal("key failure modified committed bytes")
	}
}

func TestWindowsPersonalSourceCaseAliasKeepsIdentity(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows filesystem alias contract")
	}
	s := New(t.TempDir(), nil)
	first := snapshotWithProfile(t, s, "same preference")
	upper := filepath.Join(s.ContextDir(), memory.ProfileFileName)
	lower := filepath.Join(s.ContextDir(), strings.ToLower(memory.ProfileFileName))
	if err := os.Rename(upper, lower); err != nil {
		t.Fatal(err)
	}
	second, err := s.BuildPromptSnapshot()
	if err != nil || first != second {
		t.Fatalf("case-only rename rotated identity: %v", err)
	}
}
