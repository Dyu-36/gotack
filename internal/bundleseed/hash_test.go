package bundleseed

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyIfChangedReplacesSameSizeManagedContent(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source")
	destination := filepath.Join(t.TempDir(), "destination")
	asset := filepath.Join(source, "asset.txt")
	writeTestFile(t, asset, "alpha")

	if err := CopyIfChanged(source, destination, Options{ExistingFiles: ManagedFiles}); err != nil {
		t.Fatalf("first copy: %v", err)
	}
	writeTestFile(t, asset, "bravo")
	if err := CopyIfChanged(source, destination, Options{ExistingFiles: ManagedFiles}); err != nil {
		t.Fatalf("second copy: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(destination, "asset.txt"))
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(data) != "bravo" {
		t.Fatalf("destination = %q, want %q", data, "bravo")
	}

	reportData, err := os.ReadFile(filepath.Join(destination, reportFileName))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var state report
	if err := json.Unmarshal(reportData, &state); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if state.Hashes["asset.txt"] == "" {
		t.Fatal("report did not persist a content hash")
	}
}

func TestCopyIfChangedUpgradesLegacyManagedReportByContent(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source")
	destination := filepath.Join(t.TempDir(), "destination")
	writeTestFile(t, filepath.Join(source, "asset.txt"), "newer")
	writeTestFile(t, filepath.Join(destination, "asset.txt"), "older")
	writeTestFile(t, filepath.Join(destination, reportFileName), `{"files":{"asset.txt":5}}`)

	if err := CopyIfChanged(source, destination, Options{ExistingFiles: ManagedFiles}); err != nil {
		t.Fatalf("CopyIfChanged: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(destination, "asset.txt"))
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(data) != "newer" {
		t.Fatalf("destination = %q, want %q", data, "newer")
	}
}
