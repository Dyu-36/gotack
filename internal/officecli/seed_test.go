package officecli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSeedCopiesBinariesAndSkills(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	dst := filepath.Join(root, "dst")
	if err := os.MkdirAll(filepath.Join(src, "skills", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "officecli.exe"), []byte("officecli"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "skills", "demo", "SKILL.md"), []byte("# demo"), 0o644); err != nil {
		t.Fatal(err)
	}

	seeder := New(dst, nil)
	if err := seeder.Seed(src); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	binaryName := "officecli"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	if _, err := os.Stat(filepath.Join(seeder.BinDir(), binaryName)); err != nil {
		t.Fatalf("binary not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(seeder.SkillsDir(), "demo", "SKILL.md")); err != nil {
		t.Fatalf("skill not copied: %v", err)
	}
	// Idempotent: a second call does not rewrite identical files.
	if err := seeder.Seed(src); err != nil {
		t.Fatalf("Seed (re-run): %v", err)
	}
}

func TestInstallPathPrependsBinDir(t *testing.T) {
	root := t.TempDir()
	seeder := New(root, nil)
	t.Setenv("PATH", "")
	seeder.InstallPath()
	updated := os.Getenv("PATH")
	parts := filepath.SplitList(updated)
	if len(parts) == 0 || !strings.EqualFold(parts[0], seeder.BinDir()) {
		t.Fatalf("bin dir not at start of PATH: %s", updated)
	}
}

func TestCrushEnvAndSkillsPath(t *testing.T) {
	root := t.TempDir()
	seeder := New(root, nil)
	t.Setenv("PATH", "")
	seeder.InstallPath()
	env := seeder.CrushEnv()
	if env["PATH"] == "" || !strings.Contains(env["PATH"], seeder.BinDir()) {
		t.Fatalf("Crush env missing bin dir: %+v", env)
	}
	if seeder.SkillsPathArg() != seeder.SkillsDir() {
		t.Fatalf("SkillsPathArg should match SkillsDir")
	}
}
