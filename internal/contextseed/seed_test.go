package contextseed

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Dyu-36/gotack/internal/assistant"
	"github.com/Dyu-36/gotack/internal/memory"
)

func writeSource(t *testing.T, root, name, content string) {
	t.Helper()
	writeContextFixture(t, filepath.Join(root, filepath.FromSlash(name)), content)
}

func readSeeded(t *testing.T, s *Seeder, name string) string {
	t.Helper()
	return string(mustRead(t, filepath.Join(s.ContextDir(), filepath.FromSlash(name))))
}

func TestSeedUsesEmbeddedCoreAndPreservesPersonalFiles(t *testing.T) {
	s := New(t.TempDir(), nil)
	source := t.TempDir()
	writeSource(t, source, "TACK_CORE.md", "external product override")
	writeSource(t, source, "USER.md", "external user template")
	if err := s.Seed(source); err != nil {
		t.Fatal(err)
	}
	if s.ContextDir() != memory.Directory(s.dataDir) {
		t.Fatal("personal context uses a different directory than the memory service")
	}
	writeSource(t, s.ContextDir(), memory.ProfileFileName, "My preference")
	if err := s.Seed(source); err != nil {
		t.Fatal(err)
	}
	if got := readSeeded(t, s, memory.ProfileFileName); got != "My preference" {
		t.Fatalf("existing profile was overwritten: %q", got)
	}
	if _, err := os.Stat(filepath.Join(s.ContextDir(), "TACK_CORE.md")); !os.IsNotExist(err) {
		t.Fatal("external core was copied into personal data")
	}
	gen, err := s.BuildPromptSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if got := string(mustRead(t, filepath.Join(gen, managedCoreName))); got != assistant.CorePrompt {
		t.Fatal("snapshot did not use the embedded core")
	}
}

func TestSeedFreshInstallNeedsNoResourceBundle(t *testing.T) {
	s := New(t.TempDir(), nil)
	if err := s.Seed(""); err != nil {
		t.Fatal(err)
	}
	if err := New(s.dataDir, nil).Seed(filepath.Join(t.TempDir(), "missing")); err != nil {
		t.Fatal(err)
	}
	if s.SnapshotOwner() != "managed" {
		t.Fatal("fresh personal assistant has no managed identity")
	}
	if _, err := s.BuildPromptSnapshot(); err != nil {
		t.Fatal(err)
	}
}
