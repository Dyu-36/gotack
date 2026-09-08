package office

import (
	"fmt"
	"path/filepath"
	"strings"
)

const readMaxChars = 120_000

type Kind string

const (
	KindDocx Kind = "docx"
	KindXlsx Kind = "xlsx"
	KindPptx Kind = "pptx"
)

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
