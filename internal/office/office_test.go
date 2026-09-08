package office

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func writeTestPackage(path string, parts map[string]string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	zipper := zip.NewWriter(file)
	for name, content := range parts {
		entry, err := zipper.Create(name)
		if err != nil {
			_ = zipper.Close()
			_ = file.Close()
			return err
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			_ = zipper.Close()
			_ = file.Close()
			return err
		}
	}
	if err := zipper.Close(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func TestDocxReadAndInfo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.docx")
	doc := `<document><body>` +
		`<p><r><t>Quarterly Report</t></r></p>` +
		`<p><r><t>Revenue grew fast.</t></r></p>` +
		`<tbl><tr><tc><p><r><t>Item</t></r></p></tc><tc><p><r><t>Amount</t></r></p></tc></tr></tbl>` +
		`</body></document>`
	if err := writeTestPackage(path, map[string]string{documentXML: doc}); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	info, err := Info(path)
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if !strings.Contains(info, "Word document: 2 paragraphs (1 table rows)") {
		t.Fatalf("Info() = %q", info)
	}

	content, err := Read(path, "")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	for _, want := range []string{"Quarterly Report", "Revenue grew fast.", "Item | Amount"} {
		if !strings.Contains(content, want) {
			t.Fatalf("Read() missing %q in:\n%s", want, content)
		}
	}
}

func TestPptxReadAndInfo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deck.pptx")
	parts := map[string]string{
		"ppt/slides/slide1.xml": `<slide><p><r><t>Overview</t></r></p><p><r><t>Goal one</t></r></p></slide>`,
		"ppt/slides/slide2.xml": `<slide><p><r><t>Budget</t></r></p><p><r><t>Costs rose sharply</t></r></p></slide>`,
	}
	if err := writeTestPackage(path, parts); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	info, err := Info(path)
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if !strings.Contains(info, "2 slides") {
		t.Fatalf("Info() = %q", info)
	}

	content, err := Read(path, "")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	for _, want := range []string{"## Slide 1", "Overview", "Goal one", "## Slide 2", "Budget"} {
		if !strings.Contains(content, want) {
			t.Fatalf("Read() missing %q in:\n%s", want, content)
		}
	}
}

func TestXlsxReadAndInfo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.xlsx")
	file := excelize.NewFile()
	defer file.Close()
	for cell, value := range map[string]any{
		"A1": "Name", "B1": "Qty",
		"A2": "Rice", "B2": 2,
		"A3": "Tea", "B3": true,
	} {
		if err := file.SetCellValue("Sheet1", cell, value); err != nil {
			t.Fatalf("SetCellValue(%s): %v", cell, err)
		}
	}
	if err := file.SaveAs(path); err != nil {
		t.Fatalf("SaveAs: %v", err)
	}

	info, err := Info(path)
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if !strings.Contains(info, `"Sheet1" 3x2`) {
		t.Fatalf("Info() = %q", info)
	}

	content, err := Read(path, "")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !strings.Contains(content, "Name\tQty\nRice\t2\nTea\tTRUE") {
		t.Fatalf("Read() =\n%s", content)
	}
}

func TestMalformedOOXMLReturnsError(t *testing.T) {
	for _, tc := range []struct {
		name  string
		ext   string
		parts map[string]string
	}{
		{
			name:  "docx malformed text element",
			ext:   ".docx",
			parts: map[string]string{documentXML: `<document><p><t>partial</p></document>`},
		},
		{
			name:  "pptx truncated slide",
			ext:   ".pptx",
			parts: map[string]string{"ppt/slides/slide1.xml": `<slide><p><t>partial</t></p>`},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "broken"+tc.ext)
			if err := writeTestPackage(path, tc.parts); err != nil {
				t.Fatalf("write fixture: %v", err)
			}
			if _, err := Read(path, ""); err == nil {
				t.Fatal("Read() accepted malformed XML")
			} else {
				var syntaxError *xml.SyntaxError
				if !errors.As(err, &syntaxError) {
					t.Fatalf("Read() error = %v, want *xml.SyntaxError", err)
				}
			}
		})
	}
}

func TestKindOfRejectsUnknownExtension(t *testing.T) {
	if _, err := KindOf("file.pdf"); err == nil {
		t.Fatal("expected error for unsupported extension")
	}
}
