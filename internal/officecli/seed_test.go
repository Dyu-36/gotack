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
	binaryName := "officecli"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	if err := os.MkdirAll(filepath.Join(src, "skills", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, binaryName), []byte("officecli"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "skills", "demo", "SKILL.md"), []byte("# demo"), 0o644); err != nil {
		t.Fatal(err)
	}

	seeder := New(dst, nil)
	if err := seeder.Seed(src); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(seeder.BinDir(), binaryName)); err != nil {
		t.Fatalf("binary not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(seeder.SkillsDir(), "demo", "SKILL.md")); err != nil {
		t.Fatalf("skill not copied: %v", err)
	}

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

func TestSeedRejectsMalformedReportBeforeReplacingRuntime(t *testing.T) {
	source := t.TempDir()
	seeder := New(t.TempDir(), nil)
	if err := os.WriteFile(filepath.Join(source, "officecli.exe"), []byte("bundled replacement"), 0o755); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := os.MkdirAll(seeder.BinDir(), 0o755); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	runtimePath := filepath.Join(seeder.BinDir(), "officecli.exe")
	if err := os.WriteFile(runtimePath, []byte("keep installed runtime"), 0o755); err != nil {
		t.Fatalf("write installed runtime: %v", err)
	}
	reportPath := filepath.Join(seeder.BinDir(), ".seed-report.json")
	if err := os.WriteFile(reportPath, []byte(`{"files":`), 0o644); err != nil {
		t.Fatalf("write malformed report: %v", err)
	}

	err := seeder.Seed(source)
	if err == nil || !strings.Contains(err.Error(), "parse "+reportPath) {
		t.Fatalf("Seed error = %v, want malformed report diagnostic", err)
	}
	data, readErr := os.ReadFile(runtimePath)
	if readErr != nil {
		t.Fatalf("read runtime: %v", readErr)
	}
	if string(data) != "keep installed runtime" {
		t.Fatalf("runtime changed after malformed report: %q", data)
	}
}

func TestSeedPrunesRemovedSkillAssets(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	seeder := New(filepath.Join(root, "data"), nil)
	binaryName := "officecli"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	if err := os.MkdirAll(filepath.Join(source, "skills", "timetable"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, binaryName), []byte("officecli"), 0o755); err != nil {
		t.Fatal(err)
	}
	staleSource := filepath.Join(source, "skills", "timetable", "legacy.py")
	if err := os.WriteFile(staleSource, []byte("legacy"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := seeder.Seed(source); err != nil {
		t.Fatalf("first Seed: %v", err)
	}
	if err := os.Remove(staleSource); err != nil {
		t.Fatal(err)
	}
	if err := seeder.Seed(source); err != nil {
		t.Fatalf("second Seed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(seeder.SkillsDir(), "timetable", "legacy.py")); !os.IsNotExist(err) {
		t.Fatalf("stale installed skill file still exists: %v", err)
	}
}
