package office

import (
	"strings"
	"testing"
)

func TestParseBlocksAndInline(t *testing.T) {
	src := "# Report\n\nIntro **bold** and *italic* text.\n- first bullet\n- second bullet\n2. numbered\n\n| Name | Qty |\n| --- | --- |\n| Rice | 2 |\n\n---\n"
	blocks := ParseBlocks(src)

	want := []Block{
		{Kind: kindHeading, Level: 1, Text: "Report"},
		{Kind: kindParagraph, Text: "Intro **bold** and *italic* text."},
		{Kind: kindBullet, Text: "first bullet"},
		{Kind: kindBullet, Text: "second bullet"},
		{Kind: kindNumber, Text: "numbered"},
		{Kind: kindTable, Rows: []string{"Name\tQty", "Rice\t2"}},
		{Kind: kindDivider},
	}
	if len(blocks) != len(want) {
		t.Fatalf("got %d blocks %+v, want %d", len(blocks), blocks, len(want))
	}
	for i, block := range want {
		if blocks[i].Kind != block.Kind || blocks[i].Text != block.Text || blocks[i].Level != block.Level {
			t.Fatalf("block %d = %+v, want %+v", i, blocks[i], block)
		}
	}
	if len(blocks[5].Rows) != 2 || blocks[5].Rows[1] != "Rice\t2" {
		t.Fatalf("table rows = %v", blocks[5].Rows)
	}

	spans := Inline("Intro **bold** and *italic*")
	if len(spans) != 4 || spans[0].Text != "Intro " || spans[1].Text != "bold" || !spans[1].Bold || spans[3].Text != "italic" || !spans[3].Italic {
		t.Fatalf("unexpected spans %+v", spans)
	}
}

func TestDocxRoundTrip(t *testing.T) {
	path := t.TempDir() + "/report.docx"
	source := "# Quarterly Report\n\nRevenue grew **fast**.\n- Q1 note\n\n| Item | Amount |\n| A | 10 |\n"
	if _, err := Create(CreateRequest{Path: path, Content: source}); err != nil {
		t.Fatalf("create: %v", err)
	}

	info, err := Info(path)
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	if !strings.Contains(info, "Word document") {
		t.Fatalf("info = %q", info)
	}

	content, err := Read(path, "")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	for _, want := range []string{"Quarterly Report", "Revenue grew fast.", "•  Q1 note", "Item | Amount"} {
		if !strings.Contains(content, want) {
			t.Fatalf("read output missing %q in:\n%s", want, content)
		}
	}

	replaced, err := Edit(EditRequest{Op: "replace_text", Path: path, Find: "fast", Replace: "steadily"})
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	if !strings.Contains(replaced, "1 occurrence") {
		t.Fatalf("edit report = %q", replaced)
	}
	updated, _ := Read(path, "")
	if strings.Contains(updated, "fast") || !strings.Contains(updated, "steadily") {
		t.Fatalf("replace did not apply:\n%s", updated)
	}
}

func TestDocxReplaceMissingTextFails(t *testing.T) {
	path := t.TempDir() + "/a.docx"
	if _, err := Create(CreateRequest{Path: path, Content: "# A\n\nbody"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := Edit(EditRequest{Op: "replace_text", Path: path, Find: "missing", Replace: "x"}); err == nil {
		t.Fatal("expected error for missing find text")
	}
}

func TestPptxRoundTrip(t *testing.T) {
	path := t.TempDir() + "/deck.pptx"
	source := "# Overview\n\nWelcome to the review\n- Goal one\n\n# Budget\n\nCosts rose *sharply*\n"
	if _, err := Create(CreateRequest{Path: path, Content: source}); err != nil {
		t.Fatalf("create: %v", err)
	}

	info, err := Info(path)
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	if !strings.Contains(info, "2 slides") {
		t.Fatalf("info = %q", info)
	}

	content, err := Read(path, "")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	for _, want := range []string{"## Slide 1", "Overview", "•  Goal one", "## Slide 2", "Budget"} {
		if !strings.Contains(content, want) {
			t.Fatalf("read output missing %q in:\n%s", want, content)
		}
	}

	if _, err := Edit(EditRequest{Op: "replace_text", Path: path, Find: "sharply", Replace: "slowly"}); err != nil {
		t.Fatalf("edit: %v", err)
	}
	updated, _ := Read(path, "")
	if strings.Contains(updated, "sharply") || !strings.Contains(updated, "slowly") {
		t.Fatalf("replace did not apply:\n%s", updated)
	}
}

func TestPptxCreateWithoutHeading(t *testing.T) {
	path := t.TempDir() + "/notes.pptx"
	summary, err := Create(CreateRequest{Path: path, Content: "just text"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !strings.Contains(summary, "1 slides") && !strings.Contains(summary, "1 slide") {
		t.Fatalf("summary = %q", summary)
	}

	if _, err := Create(CreateRequest{Path: t.TempDir() + "/empty.pptx", Content: "  "}); err == nil {
		t.Fatal("expected error for empty content")
	}
}

func TestXlsxRoundTrip(t *testing.T) {
	path := t.TempDir() + "/data.xlsx"
	source := "Name\tQty\nRice\t2\nTea\ttrue\n"
	if _, err := Create(CreateRequest{Path: path, Content: source}); err != nil {
		t.Fatalf("create: %v", err)
	}

	info, err := Info(path)
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	if !strings.Contains(info, `"Sheet1" 3x2`) {
		t.Fatalf("info = %q", info)
	}

	content, err := Read(path, "")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// Excel renders booleans uppercase; coercion must survive the round trip.
	if !strings.Contains(content, "Name\tQty\nRice\t2\nTea\tTRUE") {
		t.Fatalf("read output =\n%s", content)
	}

	if _, err := Edit(EditRequest{Op: "set_cell", Path: path, Cell: "B2", Value: "5"}); err != nil {
		t.Fatalf("set_cell: %v", err)
	}
	if _, err := Edit(EditRequest{Op: "append_rows", Path: path, Rows: "Sugar\t1"}); err != nil {
		t.Fatalf("append_rows: %v", err)
	}
	updated, _ := Read(path, "")
	if !strings.Contains(updated, "Rice\t5") || !strings.Contains(updated, "Sugar\t1") {
		t.Fatalf("edits did not apply:\n%s", updated)
	}
}

func TestKindOfRejectsUnknownExtension(t *testing.T) {
	if _, err := KindOf("file.pdf"); err == nil {
		t.Fatal("expected error for unsupported extension")
	}
}
