package contextseed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSource(t *testing.T, sourceDir, rel, content string) {
	t.Helper()
	target := filepath.Join(sourceDir, rel)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("create source dir: %v", err)
	}
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
}

func readSeeded(t *testing.T, seeder *Seeder, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(seeder.ContextDir(), rel))
	if err != nil {
		t.Fatalf("read seeded file: %v", err)
	}
	return string(data)
}

func TestSeed(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T, sourceDir string, seeder *Seeder)
	}{
		{
			name: "fresh seed copies every bundled file",
			run: func(t *testing.T, sourceDir string, seeder *Seeder) {
				writeSource(t, sourceDir, "TACK.md", "persona v1")
				writeSource(t, sourceDir, "memory/MEMORY.md", "facts v1")

				if err := seeder.Seed(sourceDir); err != nil {
					t.Fatalf("Seed: %v", err)
				}

				if got := readSeeded(t, seeder, "TACK.md"); got != "persona v1" {
					t.Errorf("TACK.md = %q, want persona v1", got)
				}
				if got := readSeeded(t, seeder, "memory/MEMORY.md"); got != "facts v1" {
					t.Errorf("memory/MEMORY.md = %q, want facts v1", got)
				}
				if _, err := os.Stat(filepath.Join(seeder.ContextDir(), ".seed-report.json")); err != nil {
					t.Errorf("seed report missing: %v", err)
				}
			},
		},
		{
			name: "re-seed without changes is idempotent",
			run: func(t *testing.T, sourceDir string, seeder *Seeder) {
				writeSource(t, sourceDir, "TACK.md", "persona v1")

				if err := seeder.Seed(sourceDir); err != nil {
					t.Fatalf("first Seed: %v", err)
				}
				reportBefore, err := os.ReadFile(filepath.Join(seeder.ContextDir(), ".seed-report.json"))
				if err != nil {
					t.Fatalf("read report: %v", err)
				}
				if err := seeder.Seed(sourceDir); err != nil {
					t.Fatalf("second Seed: %v", err)
				}
				reportAfter, err := os.ReadFile(filepath.Join(seeder.ContextDir(), ".seed-report.json"))
				if err != nil {
					t.Fatalf("read report: %v", err)
				}

				if got := readSeeded(t, seeder, "TACK.md"); got != "persona v1" {
					t.Errorf("TACK.md = %q after re-seed, want persona v1", got)
				}
				if string(reportBefore) != string(reportAfter) {
					t.Errorf("report changed on idempotent re-seed: %s -> %s", reportBefore, reportAfter)
				}
			},
		},
		{
			name: "user-modified file is preserved even when the bundle updates",
			run: func(t *testing.T, sourceDir string, seeder *Seeder) {
				writeSource(t, sourceDir, "TACK.md", "persona v1")
				if err := seeder.Seed(sourceDir); err != nil {
					t.Fatalf("first Seed: %v", err)
				}

				userEdit := filepath.Join(seeder.ContextDir(), "TACK.md")
				if err := os.WriteFile(userEdit, []byte("persona v1, user additions"), 0o644); err != nil {
					t.Fatalf("simulate user edit: %v", err)
				}

				writeSource(t, sourceDir, "TACK.md", "persona v2 with different length")

				if err := seeder.Seed(sourceDir); err != nil {
					t.Fatalf("second Seed: %v", err)
				}
				if got := readSeeded(t, seeder, "TACK.md"); got != "persona v1, user additions" {
					t.Errorf("TACK.md = %q, want the user edit preserved", got)
				}
			},
		},
		{
			name: "updated bundled file propagates when the destination is untouched",
			run: func(t *testing.T, sourceDir string, seeder *Seeder) {
				writeSource(t, sourceDir, "TACK.md", "persona v1")
				if err := seeder.Seed(sourceDir); err != nil {
					t.Fatalf("first Seed: %v", err)
				}
				writeSource(t, sourceDir, "TACK.md", "persona v2 with different length")

				if err := seeder.Seed(sourceDir); err != nil {
					t.Fatalf("second Seed: %v", err)
				}
				if got := readSeeded(t, seeder, "TACK.md"); got != "persona v2 with different length" {
					t.Errorf("TACK.md = %q, want the updated bundle propagated", got)
				}
			},
		},
		{
			name: "pre-existing user file is never overwritten on first seed",
			run: func(t *testing.T, sourceDir string, seeder *Seeder) {
				if err := os.MkdirAll(seeder.ContextDir(), 0o755); err != nil {
					t.Fatalf("create context dir: %v", err)
				}
				userFile := filepath.Join(seeder.ContextDir(), "TACK.md")
				if err := os.WriteFile(userFile, []byte("hand-written preferences"), 0o644); err != nil {
					t.Fatalf("write user file: %v", err)
				}
				writeSource(t, sourceDir, "TACK.md", "persona v1")

				if err := seeder.Seed(sourceDir); err != nil {
					t.Fatalf("Seed: %v", err)
				}
				if got := readSeeded(t, seeder, "TACK.md"); got != "hand-written preferences" {
					t.Errorf("TACK.md = %q, want the pre-existing user file untouched", got)
				}
			},
		},
		{
			name: "empty source dir is a no-op",
			run: func(t *testing.T, sourceDir string, seeder *Seeder) {
				if err := seeder.Seed(""); err != nil {
					t.Fatalf("Seed(\"\") = %v, want nil", err)
				}
				if _, err := os.Stat(seeder.ContextDir()); !os.IsNotExist(err) {
					t.Errorf("context dir should not be created for an empty source, stat err = %v", err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceDir := t.TempDir()
			seeder := New(t.TempDir(), nil)
			tt.run(t, sourceDir, seeder)
		})
	}
}

func TestSeedRejectsMalformedReportWithoutChangingUserFile(t *testing.T) {
	sourceDir := t.TempDir()
	seeder := New(t.TempDir(), nil)
	writeSource(t, sourceDir, "TACK.md", "bundled replacement")
	if err := os.MkdirAll(seeder.ContextDir(), 0o755); err != nil {
		t.Fatalf("create context dir: %v", err)
	}
	userPath := filepath.Join(seeder.ContextDir(), "TACK.md")
	if err := os.WriteFile(userPath, []byte("keep my context"), 0o644); err != nil {
		t.Fatalf("write user context: %v", err)
	}
	reportPath := filepath.Join(seeder.ContextDir(), ".seed-report.json")
	if err := os.WriteFile(reportPath, []byte(`{"files":`), 0o644); err != nil {
		t.Fatalf("write malformed report: %v", err)
	}

	err := seeder.Seed(sourceDir)
	if err == nil || !strings.Contains(err.Error(), "parse "+reportPath) {
		t.Fatalf("Seed error = %v, want malformed report diagnostic", err)
	}
	if got := readSeeded(t, seeder, "TACK.md"); got != "keep my context" {
		t.Fatalf("TACK.md = %q after malformed report, want user content", got)
	}
}

func TestRepoTrackedTackContext(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "resources", "context", "TACK.md"))
	if err != nil {
		t.Fatalf("resources/context/TACK.md must exist in the repo: %v", err)
	}
	text := string(data)

	if strings.Contains(text, "{{") {
		t.Errorf("TACK.md contains a Go template directive; context files are not template-executed")
	}
	if !strings.Contains(text, "Tack") {
		t.Errorf("TACK.md must name the Tack persona")
	}
	if strings.Contains(text, "permissions in auto-approved mode") {
		t.Error("TACK.md must not claim blanket auto-approval; interactive approval is the default")
	}
	for _, marker := range []string{
		"## Core Principles",
		"## Task Management",
		"### Full Filesystem and Folder Access",
		"### Desktop and Screen Capture on Windows",
		"### Automatic Media and Document Delivery",
		"## Implementation Methodology",
	} {
		if !strings.Contains(text, marker) {
			t.Errorf("TACK.md lost the parity marker %q", marker)
		}
	}
}
