package officecli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func timetableCoreRuntime(t *testing.T) (string, string) {
	t.Helper()
	root := filepath.Clean(filepath.Join("..", ".."))
	resourceRoot := filepath.Join(root, "resources")
	if packaged := os.Getenv("GOTACK_RESOURCE_ROOT"); packaged != "" {
		resourceRoot = packaged
	}
	skillDir := filepath.Join(resourceRoot, "skills", "timetable")

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
			t.Skip("Python is unavailable")
		}
	}
	return skillDir, python
}

func TestBundledTimetableCoreRunnerFilesExist(t *testing.T) {
	skillDir, _ := timetableCoreRuntime(t)
	for _, relative := range []string{
		filepath.Join("reference", "problem-schema-core.md"),
		filepath.Join("runtime", "run.py"),
	} {
		if _, err := os.Stat(filepath.Join(skillDir, relative)); err != nil {
			t.Fatalf("missing timetable core resource %s: %v", relative, err)
		}
	}
}

func TestBundledTimetableRunnerCreatesWorkbook(t *testing.T) {
	skillDir, python := timetableTestRuntime(t)
	workbook := filepath.Join(t.TempDir(), "thoi-khoa-bieu.xlsx")
	problem := filepath.Join("testdata", "timetable-problem.json")

	command := exec.Command(
		python,
		"-X", "utf8",
		filepath.Join(skillDir, "runtime", "run.py"),
		problem,
		workbook,
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("timetable runner failed: %v\n%s", err, output)
	}
	info, err := os.Stat(workbook)
	if err != nil {
		t.Fatalf("runner did not create workbook: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("runner created an empty workbook")
	}
}

func TestBundledTimetableRunnerRejectsResourceRequirements(t *testing.T) {
	skillDir, python := timetableCoreRuntime(t)
	workbook := filepath.Join(t.TempDir(), "thoi-khoa-bieu.xlsx")
	problem := filepath.Join("testdata", "timetable-requirements-problem.json")

	command := exec.Command(
		python,
		"-X", "utf8",
		filepath.Join(skillDir, "runtime", "run.py"),
		problem,
		workbook,
	)
	output, err := command.CombinedOutput()
	exitError, ok := err.(*exec.ExitError)
	if !ok || exitError.ExitCode() != 3 {
		t.Fatalf("resource requirement exit = %v, want code 3\n%s", err, output)
	}
	if !strings.Contains(string(output), "không hỗ trợ yêu cầu về phòng học") {
		t.Fatalf("resource rejection was not explained:\n%s", output)
	}
	if _, err := os.Stat(workbook); !os.IsNotExist(err) {
		t.Fatalf("resource rejection unexpectedly created output: %v", err)
	}
}
