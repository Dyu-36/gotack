package officecli

import (
	"archive/zip"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestBundledTimetableSolverAndExporter(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	resourceRoot := filepath.Join(root, "resources")
	if packaged := os.Getenv("GOTACK_RESOURCE_ROOT"); packaged != "" {
		resourceRoot = packaged
	}
	skillDir := filepath.Join(resourceRoot, "skills", "timetable")
	for _, relative := range []string{
		"SKILL.md",
		"metadata.json",
		filepath.Join("reference", "problem-schema.md"),
		filepath.Join("runtime", "solver.py"),
		filepath.Join("runtime", "exporter.py"),
	} {
		if _, err := os.Stat(filepath.Join(skillDir, relative)); err != nil {
			t.Fatalf("missing timetable resource %s: %v", relative, err)
		}
	}

	python := filepath.Join(resourceRoot, "bin", "python.exe")
	if os.Getenv("GOTACK_RESOURCE_ROOT") != "" {
		python = filepath.Join(resourceRoot, "python.exe")
	}
	if runtime.GOOS != "windows" {
		python = filepath.Join(resourceRoot, "bin", "python")
		if os.Getenv("GOTACK_RESOURCE_ROOT") != "" {
			python = filepath.Join(resourceRoot, "python")
		}
	}
	if _, err := os.Stat(python); err != nil {
		var lookupErr error
		python, lookupErr = exec.LookPath("python")
		if lookupErr != nil {
			t.Skip("Python is unavailable; resource presence was verified")
		}
	}
	if err := exec.Command(python, "-c", "import openpyxl, ortools").Run(); err != nil {
		t.Skipf("Python timetable libraries are unavailable: %v", err)
	}

	outDir := t.TempDir()
	schedule := filepath.Join(outDir, "schedule.json")
	workbook := filepath.Join(outDir, "thoi-khoa-bieu.xlsx")
	problem := filepath.Join("testdata", "timetable-problem.json")

	solver := exec.Command(python, "-X", "utf8", filepath.Join(skillDir, "runtime", "solver.py"), problem, schedule)
	if output, err := solver.CombinedOutput(); err != nil {
		t.Fatalf("timetable solver failed: %v\n%s", err, output)
	}
	raw, err := os.ReadFile(schedule)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Assignments []json.RawMessage `json:"assignments"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("invalid schedule JSON: %v", err)
	}
	if len(result.Assignments) != 2 {
		t.Fatalf("expected 2 scheduled periods, got %d", len(result.Assignments))
	}

	exporter := exec.Command(python, "-X", "utf8", filepath.Join(skillDir, "runtime", "exporter.py"), schedule, workbook)
	if output, err := exporter.CombinedOutput(); err != nil {
		t.Fatalf("timetable exporter failed: %v\n%s", err, output)
	}
	archive, err := zip.OpenReader(workbook)
	if err != nil {
		t.Fatalf("exported workbook is not a valid xlsx: %v", err)
	}
	defer archive.Close()
	foundWorkbook := false
	for _, file := range archive.File {
		if file.Name == "xl/workbook.xml" {
			foundWorkbook = true
			break
		}
	}
	if !foundWorkbook {
		t.Fatal("exported xlsx is missing xl/workbook.xml")
	}
}
