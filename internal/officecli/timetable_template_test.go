package officecli

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func timetableTemplatePath(t *testing.T) string {
	t.Helper()
	root := filepath.Clean(filepath.Join("..", ".."))
	resourceRoot := filepath.Join(root, "resources")
	if packaged := os.Getenv("GOTACK_RESOURCE_ROOT"); packaged != "" {
		resourceRoot = packaged
	}
	path := filepath.Join(resourceRoot, "skills", "timetable", "assets", "mau-thoi-khoa-bieu.xlsx")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("missing timetable template: %v", err)
	}
	return path
}
func TestBundledTimetableTemplateContract(t *testing.T) {
	path := timetableTemplatePath(t)
	book, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatalf("open timetable template: %v", err)
	}
	defer func() { _ = book.Close() }()

	if got, want := book.GetSheetList(), []string{"Thời khóa biểu", "Dữ liệu"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("sheet list = %#v, want %#v", got, want)
	}
	wantOutput := []string{"Thứ", "Tiết", "6A", "GV Dạy", "6B", "GV Dạy", "7A", "GV Dạy", "7B", "GV Dạy", "8A", "GV Dạy", "8B", "GV Dạy", "9A", "GV Dạy", "9B", "GV Dạy"}
	gotOutput, err := book.GetRows("Thời khóa biểu")
	if err != nil || len(gotOutput) < 3 || !reflect.DeepEqual(gotOutput[2], wantOutput) {
		t.Fatalf("output headers = %#v, err = %v", gotOutput, err)
	}
	wantData := []string{"Thứ", "Buổi", "Tiết", "Lớp", "Môn", "Giáo viên"}
	gotData, err := book.GetRows("Dữ liệu")
	if err != nil || len(gotData) == 0 || len(gotData[0]) < len(wantData) || !reflect.DeepEqual(gotData[0][:len(wantData)], wantData) {
		t.Fatalf("data headers = %#v, err = %v", gotData, err)
	}
	for _, cell := range []string{"C4", "D4", "Q25", "R25"} {
		formula, err := book.GetCellFormula("Thời khóa biểu", cell)
		if err != nil || !strings.Contains(formula, "Dữ liệu") {
			t.Fatalf("formula %s = %q, err = %v", cell, formula, err)
		}
	}
}
func readWorkbookEntry(t *testing.T, path, name string) string {
	t.Helper()
	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open xlsx archive: %v", err)
	}
	defer archive.Close()
	for _, file := range archive.File {
		if file.Name != name {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer reader.Close()
		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	t.Fatalf("xlsx entry %s not found", name)
	return ""
}

func TestBundledTimetableTemplateKeepsSuppliedFormat(t *testing.T) {
	path := timetableTemplatePath(t)
	styles := readWorkbookEntry(t, path, "xl/styles.xml")
	if !strings.Contains(styles, "Times New Roman") {
		t.Fatal("template no longer uses Times New Roman")
	}
	workbook := readWorkbookEntry(t, path, "xl/workbook.xml")
	if !strings.Contains(workbook, `fullCalcOnLoad="1"`) {
		t.Fatal("template must recalculate formulas when opened")
	}
}
