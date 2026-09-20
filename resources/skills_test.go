package resources

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestInstallBundledTimetable(t *testing.T) {
	configDir := t.TempDir()
	userSkill := filepath.Join(configDir, "skills", "timetable", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(userSkill), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userSkill, []byte("user override"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := InstallSkills(configDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"SKILL.md", "assets/mau-thoi-khoa-bieu.xlsx", "assets/phan-cong-chuan-hoa.xlsx"} {
		want, err := bundledSkills.ReadFile("skills/timetable/" + name)
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(root, "timetable", filepath.FromSlash(name)))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("resource %s differs from embedded source: %v", name, err)
		}
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			again, err := InstallSkills(configDir)
			if err != nil || again != root {
				t.Errorf("concurrent install = %q, %v; want %q", again, err, root)
			}
		})
	}
	wg.Wait()
	data, err := os.ReadFile(userSkill)
	if err != nil || string(data) != "user override" {
		t.Fatalf("user skill was modified: %q, %v", data, err)
	}
}

func TestInstallRepairsManagedFiles(t *testing.T) {
	configDir := t.TempDir()
	root, err := InstallSkills(configDir)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "timetable", "SKILL.md")
	if err := os.WriteFile(path, []byte("damaged"), 0o600); err != nil {
		t.Fatal(err)
	}
	unexpected := filepath.Join(root, "unexpected.txt")
	if err := os.WriteFile(unexpected, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	repaired, err := InstallSkills(configDir)
	if err != nil || repaired != root {
		t.Fatalf("repair = %q, %v", repaired, err)
	}
	want, _ := bundledSkills.ReadFile("skills/timetable/SKILL.md")
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("resource not repaired: %v", err)
	}
	if _, err := os.Stat(unexpected); !os.IsNotExist(err) {
		t.Fatalf("unexpected managed file was retained: %v", err)
	}
}

func TestInstallRejectsRelativeRoot(t *testing.T) {
	if _, err := InstallSkills(""); err == nil {
		t.Fatal("empty root accepted")
	}
	if _, err := InstallSkills("relative"); err == nil {
		t.Fatal("relative root accepted")
	}
}
