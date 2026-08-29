package office

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// office.go -- role: file-kind dispatch for the four MCP tool operations.
// Extension decides the implementation; content format is markdown for Word
// and PowerPoint and TSV for Excel.

// readMaxChars bounds any single read result so one call cannot flood the
// agent context.
const readMaxChars = 120_000

// Kind identifies a supported office file type.
type Kind string

// Supported file kinds.
const (
	KindDocx Kind = "docx"
	KindXlsx Kind = "xlsx"
	KindPptx Kind = "pptx"
)

// KindOf maps a file extension to its kind.
func KindOf(path string) (Kind, error) {
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(path), ".")) {
	case "docx":
		return KindDocx, nil
	case "xlsx":
		return KindXlsx, nil
	case "pptx":
		return KindPptx, nil
	default:
		return "", fmt.Errorf("office: unsupported file type %q (use .docx, .xlsx or .pptx)", filepath.Ext(path))
	}
}

// Info returns a one-line structure summary.
func Info(path string) (string, error) {
	kind, err := KindOf(path)
	if err != nil {
		return "", err
	}
	switch kind {
	case KindDocx:
		return docxInfo(path)
	case KindXlsx:
		return xlsxInfo(path)
	default:
		return pptxInfo(path)
	}
}

// Read extracts document content as text. sheet selects one Excel sheet.
func Read(path, sheet string) (string, error) {
	kind, err := KindOf(path)
	if err != nil {
		return "", err
	}
	switch kind {
	case KindDocx:
		return docxRead(path)
	case KindXlsx:
		return xlsxRead(path, sheet, readMaxChars)
	default:
		return pptxRead(path)
	}
}

// CreateRequest is the office_create tool input.
type CreateRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// Create generates a new file from content.
func Create(req CreateRequest) (string, error) {
	if req.Path == "" {
		return "", fmt.Errorf("office: path is required")
	}
	kind, err := KindOf(req.Path)
	if err != nil {
		return "", err
	}
	switch kind {
	case KindDocx:
		err = docxCreate(req.Path, req.Content)
	case KindXlsx:
		err = xlsxCreate(req.Path, req.Content)
	default:
		err = pptxCreate(req.Path, req.Content)
	}
	if err != nil {
		return "", err
	}
	summary, err := Info(req.Path)
	if err != nil {
		return fmt.Sprintf("created %s", req.Path), nil
	}
	return summary, nil
}

// EditRequest is the office_edit tool input. Op selects the operation:
// replace_text (docx/pptx), set_cell and append_rows (xlsx).
type EditRequest struct {
	Op      string `json:"op"`
	Path    string `json:"path"`
	Find    string `json:"find,omitempty"`
	Replace string `json:"replace,omitempty"`
	Sheet   string `json:"sheet,omitempty"`
	Cell    string `json:"cell,omitempty"`
	Value   string `json:"value,omitempty"`
	Rows    string `json:"rows,omitempty"`
}

// Edit applies one mutation and reports what changed.
func Edit(req EditRequest) (string, error) {
	if req.Path == "" {
		return "", fmt.Errorf("office: path is required")
	}
	kind, err := KindOf(req.Path)
	if err != nil {
		return "", err
	}
	switch req.Op {
	case "replace_text":
		if kind == KindXlsx {
			return "", fmt.Errorf("office: replace_text is not available for .xlsx; use set_cell")
		}
		var count int
		if kind == KindDocx {
			count, err = docxReplace(req.Path, req.Find, req.Replace)
		} else {
			count, err = pptxReplace(req.Path, req.Find, req.Replace)
		}
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("replaced %d occurrence(s) of %q", count, req.Find), nil
	case "set_cell":
		if kind != KindXlsx {
			return "", fmt.Errorf("office: set_cell requires a .xlsx file")
		}
		if req.Cell == "" {
			return "", fmt.Errorf("office: cell is required for set_cell")
		}
		if err := xlsxSetCell(req.Path, req.Sheet, req.Cell, req.Value); err != nil {
			return "", err
		}
		return fmt.Sprintf("set %s!%s", sheetLabel(req.Sheet), req.Cell), nil
	case "append_rows":
		if kind != KindXlsx {
			return "", fmt.Errorf("office: append_rows requires a .xlsx file")
		}
		appended, err := xlsxAppendRows(req.Path, req.Sheet, req.Rows)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("appended %d row(s) to %s", appended, sheetLabel(req.Sheet)), nil
	default:
		return "", fmt.Errorf("office: unknown op %q (use replace_text, set_cell or append_rows)", req.Op)
	}
}

// MarshalArgs decodes a tool-call arguments blob into a typed request.
func MarshalArgs[T any](raw json.RawMessage) (T, error) {
	var req T
	if len(raw) == 0 {
		return req, fmt.Errorf("office: arguments are required")
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		return req, fmt.Errorf("office: decode arguments: %w", err)
	}
	return req, nil
}

// sheetLabel names the first sheet when the caller left the sheet unset.
func sheetLabel(sheet string) string {
	if sheet == "" {
		return "(first sheet)"
	}
	return sheet
}
