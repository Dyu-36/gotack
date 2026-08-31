package attachments

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// pdf.go -- role: extract text from a PDF without adding a Go dependency.
//
// Order of attempts: pdftotext (poppler) -> LibreOffice -> Microsoft Word COM.
// All three are optional. When every attempt fails, the caller still hands the
// agent the file path plus the bundled office tooling hint, which beats the old
// behaviour where the model saw a bare path and shelled out to install packages.

// wdFormatText is Word's SaveAs2 format id for plain text.
const wdFormatText = 2

// ExtractTextFromPDF returns the plain text of a PDF file.
func ExtractTextFromPDF(path string) (string, error) {
	dir, err := os.MkdirTemp("", "gotack-pdf-")
	if err != nil {
		return "", fmt.Errorf("tạo thư mục tạm: %w", err)
	}
	defer os.RemoveAll(dir)

	target := filepath.Join(dir, "extracted.txt")
	var problems []string

	if binary, lookErr := exec.LookPath("pdftotext"); lookErr == nil {
		convErr := runConversion(binary, "-layout", "-enc", "UTF-8", path, target)
		if text, ok := readExtracted(target); ok && convErr == nil {
			return text, nil
		}
		problems = append(problems, "pdftotext: "+conversionProblem(convErr))
	}

	if binary := findSoffice(); binary != "" {
		convErr := runConversion(binary, "--headless", "--norestore", "--convert-to", "txt:Text", "--outdir", dir, path)
		candidate := filepath.Join(dir, strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))+".txt")
		if text, ok := readExtracted(candidate); ok && convErr == nil {
			return text, nil
		}
		problems = append(problems, "LibreOffice: "+conversionProblem(convErr))
	} else {
		problems = append(problems, "không tìm thấy LibreOffice (soffice)")
	}

	if runtime.GOOS == "windows" {
		convErr := convertPDFWithWordCOM(path, target)
		if text, ok := readExtracted(target); ok && convErr == nil {
			return text, nil
		}
		problems = append(problems, "Microsoft Word COM: "+conversionProblem(convErr))
	}
	return "", errors.New(strings.Join(problems, "; "))
}

// readExtracted reads a converter's output through the charset decoder: Word and
// LibreOffice both emit UTF-16 or CP1252 text on Vietnamese Windows.
func readExtracted(path string) (string, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	text, _ := DecodeText(raw)
	if strings.TrimSpace(text) == "" {
		return "", false
	}
	return text, true
}

func conversionProblem(err error) string {
	if err == nil {
		return "không trích xuất được văn bản"
	}
	return err.Error()
}

// convertPDFWithWordCOM drives an installed Word through PowerShell. Word opens
// PDFs natively; ConfirmConversions=$false suppresses its conversion prompt.
func convertPDFWithWordCOM(src, dst string) error {
	script := fmt.Sprintf(`$ErrorActionPreference='Stop';$app=New-Object -ComObject Word.Application;$app.Visible=$false;$app.DisplayAlerts=0;try{$doc=$app.Documents.Open(%s,$false,$true);$doc.SaveAs2(%s,%d);$doc.Close($false)}finally{$app.Quit()}`, psQuote(src), psQuote(dst), wdFormatText)
	return runConversion("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
}
