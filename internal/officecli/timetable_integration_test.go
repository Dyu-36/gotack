package officecli

import (
	"archive/zip"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func timetableTestRuntime(t *testing.T) (string, string) {
	t.Helper()
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
		filepath.Join("runtime", "timetable_model.py"),
		filepath.Join("runtime", "timetable_requirements.py"),
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
	return skillDir, python
}

func TestBundledTimetableSolverAndExporter(t *testing.T) {
	skillDir, python := timetableTestRuntime(t)

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

func TestBundledTimetableRequirementRegistry(t *testing.T) {
	skillDir, python := timetableTestRuntime(t)
	schedule := filepath.Join(t.TempDir(), "schedule.json")
	problem := filepath.Join("testdata", "timetable-requirements-problem.json")

	command := exec.Command(
		python,
		"-X", "utf8",
		filepath.Join(skillDir, "runtime", "solver.py"),
		problem,
		schedule,
		"--phase-a-only",
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("timetable requirement solve failed: %v\n%s", err, output)
	}

	var result struct {
		Assignments []struct {
			Period  int      `json:"period"`
			Subject string   `json:"subject"`
			Labels  []string `json:"labels"`
		} `json:"assignments"`
	}
	raw, err := os.ReadFile(schedule)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("invalid schedule JSON: %v", err)
	}
	if len(result.Assignments) != 2 {
		t.Fatalf("expected 2 scheduled periods, got %d", len(result.Assignments))
	}
	for _, assignment := range result.Assignments {
		switch assignment.Subject {
		case "Toán":
			if assignment.Period != 1 {
				t.Fatalf("pinned Toán period = %d, want 1", assignment.Period)
			}
		case "Khoa học":
			if assignment.Period != 2 || len(assignment.Labels) != 1 || assignment.Labels[0] != "Phòng lab" {
				t.Fatalf("resource assignment = %#v, want period 2 with Phòng lab", assignment)
			}
		default:
			t.Fatalf("unexpected subject %q", assignment.Subject)
		}
	}
}

func TestBundledTimetableRejectsMalformedProblems(t *testing.T) {
	skillDir, python := timetableTestRuntime(t)
	tests := []struct {
		name     string
		file     string
		wantText string
	}{
		{
			name:     "unknown requirement",
			file:     "timetable-malformed-problem.json",
			wantText: "loại không hỗ trợ",
		},
		{
			name:     "nested value shapes",
			file:     "timetable-malformed-shapes-problem.json",
			wantText: "frame.days[0].sessions[0] phải là đối tượng",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schedule := filepath.Join(t.TempDir(), "schedule.json")
			problem := filepath.Join("testdata", tt.file)
			command := exec.Command(
				python,
				"-X", "utf8",
				filepath.Join(skillDir, "runtime", "solver.py"),
				problem,
				schedule,
				"--phase-a-only",
			)
			output, err := command.CombinedOutput()
			exitError, ok := err.(*exec.ExitError)
			if !ok || exitError.ExitCode() != 3 {
				t.Fatalf("malformed problem exit = %v, want code 3\n%s", err, output)
			}
			if !strings.Contains(string(output), tt.wantText) {
				t.Fatalf("malformed problem did not explain %q:\n%s", tt.wantText, output)
			}
			if _, err := os.Stat(schedule); !os.IsNotExist(err) {
				t.Fatalf("malformed problem unexpectedly created output: %v", err)
			}
		})
	}
}
