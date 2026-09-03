package bundleseed

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestCopyIfChangedExistingFilePolicies(t *testing.T) {
	for _, tc := range []struct {
		name       string
		policy     ExistingFilePolicy
		want       string
		wantReason PreserveReason
		preserved  bool
	}{
		{name: "managed file is replaced", policy: ManagedFiles, want: "bundled"},
		{name: "user file is preserved", policy: UserEditableFiles, want: "user", wantReason: UntrackedFile, preserved: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := filepath.Join(t.TempDir(), "source")
			destination := filepath.Join(t.TempDir(), "destination")
			writeTestFile(t, filepath.Join(source, "asset.txt"), "bundled")
			writeTestFile(t, filepath.Join(destination, "asset.txt"), "user")

			var gotReason PreserveReason
			gotPreserved := false
			err := CopyIfChanged(source, destination, Options{
				ExistingFiles: tc.policy,
				OnPreserve: func(_ string, reason PreserveReason) {
					gotPreserved = true
					gotReason = reason
				},
			})
			if err != nil {
				t.Fatalf("CopyIfChanged: %v", err)
			}
			data, err := os.ReadFile(filepath.Join(destination, "asset.txt"))
			if err != nil {
				t.Fatalf("read destination: %v", err)
			}
			if string(data) != tc.want {
				t.Fatalf("destination = %q, want %q", data, tc.want)
			}
			if gotPreserved != tc.preserved || gotPreserved && gotReason != tc.wantReason {
				t.Fatalf("preservation = %v reason %v, want %v reason %v", gotPreserved, gotReason, tc.preserved, tc.wantReason)
			}
		})
	}
}

func TestCopyIfChangedRejectsMalformedReportBeforeCopy(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source")
	destination := filepath.Join(t.TempDir(), "destination")
	writeTestFile(t, filepath.Join(source, "asset.txt"), "bundled replacement")
	writeTestFile(t, filepath.Join(destination, "asset.txt"), "keep me")
	reportPath := filepath.Join(destination, reportFileName)
	writeTestFile(t, reportPath, `{"files":`)

	err := CopyIfChanged(source, destination, Options{ExistingFiles: ManagedFiles})
	if err == nil || !strings.Contains(err.Error(), "parse "+reportPath) {
		t.Fatalf("CopyIfChanged error = %v, want malformed report diagnostic", err)
	}
	data, readErr := os.ReadFile(filepath.Join(destination, "asset.txt"))
	if readErr != nil {
		t.Fatalf("read destination: %v", readErr)
	}
	if string(data) != "keep me" {
		t.Fatalf("destination changed after malformed report: %q", data)
	}
	reportData, readErr := os.ReadFile(reportPath)
	if readErr != nil {
		t.Fatalf("read report: %v", readErr)
	}
	if string(reportData) != `{"files":` {
		t.Fatalf("malformed report was replaced: %q", reportData)
	}
}

func TestCopyIfChangedAtomicallyReplacesReport(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source")
	destination := filepath.Join(t.TempDir(), "destination")
	asset := filepath.Join(source, "asset.txt")
	writeTestFile(t, asset, "v1")
	if err := CopyIfChanged(source, destination, Options{ExistingFiles: ManagedFiles}); err != nil {
		t.Fatalf("first copy: %v", err)
	}
	writeTestFile(t, asset, "version two")
	if err := CopyIfChanged(source, destination, Options{ExistingFiles: ManagedFiles}); err != nil {
		t.Fatalf("second copy: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(destination, reportFileName))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var state report
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("report is not valid JSON: %v", err)
	}
	if got, want := state.Files["asset.txt"], int64(len("version two")); got != want {
		t.Fatalf("reported size = %d, want %d", got, want)
	}
	leftovers, err := filepath.Glob(filepath.Join(destination, ".seed-report-*.tmp"))
	if err != nil {
		t.Fatalf("glob report temps: %v", err)
	}
	if len(leftovers) != 0 {
		t.Fatalf("temporary reports left behind: %v", leftovers)
	}
}

func TestCopyIfChangedPrunesRemovedManagedFile(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source")
	destination := filepath.Join(t.TempDir(), "destination")
	asset := filepath.Join(source, "runtime", "solver.py")
	writeTestFile(t, asset, "legacy")
	if err := CopyIfChanged(source, destination, Options{ExistingFiles: ManagedFiles}); err != nil {
		t.Fatalf("first copy: %v", err)
	}
	if err := os.Remove(asset); err != nil {
		t.Fatal(err)
	}
	if err := CopyIfChanged(source, destination, Options{ExistingFiles: ManagedFiles}); err != nil {
		t.Fatalf("second copy: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, "runtime", "solver.py")); !os.IsNotExist(err) {
		t.Fatalf("stale managed file still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, "runtime")); !os.IsNotExist(err) {
		t.Fatalf("empty managed directory still exists: %v", err)
	}
}

func TestCopyIfChangedPreservesModifiedRemovedUserFile(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source")
	destination := filepath.Join(t.TempDir(), "destination")
	asset := filepath.Join(source, "skill", "SKILL.md")
	writeTestFile(t, asset, "bundled")
	if err := CopyIfChanged(source, destination, Options{ExistingFiles: UserEditableFiles}); err != nil {
		t.Fatalf("first copy: %v", err)
	}
	target := filepath.Join(destination, "skill", "SKILL.md")
	writeTestFile(t, target, "user edit")
	if err := os.Remove(asset); err != nil {
		t.Fatal(err)
	}
	preserved := false
	if err := CopyIfChanged(source, destination, Options{
		ExistingFiles: UserEditableFiles,
		OnPreserve: func(path string, reason PreserveReason) {
			preserved = path == filepath.Join("skill", "SKILL.md") && reason == ModifiedFile
		},
	}); err != nil {
		t.Fatalf("second copy: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "user edit" || !preserved {
		t.Fatalf("modified removed file was not preserved: %q, callback=%v", data, preserved)
	}
}
