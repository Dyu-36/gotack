//go:build windows

package officecli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestOfficeCLIImportPersistsAcrossProcessClose(t *testing.T) {
	exe := filepath.Join("..", "..", "resources", "bin", "officecli.exe")
	if _, err := os.Stat(exe); err != nil {
		t.Skipf("bundled officecli unavailable: %v", err)
	}
	dir := filepath.Join(t.TempDir(), "đường dẫn có khoảng trắng")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	workbook := filepath.Join(dir, "phân công chuẩn hóa.xlsx")
	t.Cleanup(func() { _ = exec.Command(exe, "close", workbook).Run() })
	if version := strings.TrimSpace(runOfficeCLI(t, exe, "--version")); version != "1.0.147" {
		t.Fatalf("officecli version = %q, want 1.0.147", version)
	}
	assignments := filepath.Join(dir, "phân công.csv")
	checks := filepath.Join(dir, "đối soát.csv")
	writeCSV(t, assignments, "Tên giáo viên,Môn,Lớp,Số tiết cần dạy trong tuần\nNguyễn Văn An,Toán,7A,4\nTrần Minh,Ngữ văn,7B,4\n")
	writeCSV(t, checks, "Mục,Giá trị\nTổng tiết,8\n")

	runOfficeCLI(t, exe, "create", workbook)
	runOfficeCLI(t, exe, "close", workbook)
	runOfficeCLI(t, exe, "import", workbook, "/Sheet1", assignments, "--header")
	runOfficeCLI(t, exe, "set", workbook, "/Sheet1", "--prop", "name=Phân công")
	runOfficeCLI(t, exe, "close", workbook)
	runOfficeCLI(t, exe, "add", workbook, "/", "--type", "sheet", "--prop", "name=Đối soát")
	runOfficeCLI(t, exe, "close", workbook)
	runOfficeCLI(t, exe, "import", workbook, "/Đối soát", checks, "--header")
	runOfficeCLI(t, exe, "close", workbook)

	assignmentOutput := runOfficeCLI(t, exe, "get", workbook, "/Phân công/A1:D3", "--json")
	checkOutput := runOfficeCLI(t, exe, "get", workbook, "/Đối soát/A1:B2", "--json")
	for _, want := range []string{"Tên giáo viên", "Nguyễn Văn An", "Ngữ văn", "7B"} {
		if !strings.Contains(assignmentOutput, want) {
			t.Fatalf("assignment readback missing %q: %s", want, assignmentOutput)
		}
	}
	for _, want := range []string{"Tổng tiết", "8"} {
		if !strings.Contains(checkOutput, want) {
			t.Fatalf("check readback missing %q: %s", want, checkOutput)
		}
	}
}

func writeCSV(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runOfficeCLI(t *testing.T, exe string, args ...string) string {
	t.Helper()
	command := exec.Command(exe, args...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("officecli %v: %v\n%s", args, err, output)
	}
	return string(output)
}
