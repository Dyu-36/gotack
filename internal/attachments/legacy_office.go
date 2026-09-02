package attachments

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Dyu-36/gotack/internal/userstrings"
)

const conversionTimeout = 2 * time.Minute

var ooxmlTargets = map[string]string{
	".xls":  ".xlsx",
	".xlsm": ".xlsx",
	".xlsb": ".xlsx",
	".ods":  ".xlsx",
	".doc":  ".docx",
	".rtf":  ".docx",
	".odt":  ".docx",
	".ppt":  ".pptx",
	".pps":  ".pptx",
	".odp":  ".pptx",
}

func ConvertLegacyOffice(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	target, ok := ooxmlTargets[ext]
	if !ok {
		return "", fmt.Errorf(userstrings.FmtUnsupportedConversion, ext)
	}
	converted := strings.TrimSuffix(path, filepath.Ext(path)) + target
	if info, err := os.Stat(converted); err == nil && info.Size() > 0 {
		return converted, nil
	}

	var problems []string
	if binary := findSoffice(); binary != "" {
		err := runConversion(binary, "--headless", "--norestore", "--convert-to",
			strings.TrimPrefix(target, "."), "--outdir", filepath.Dir(converted), path)
		if err == nil {
			if info, statErr := os.Stat(converted); statErr == nil && info.Size() > 0 {
				return converted, nil
			}
			err = errors.New(userstrings.ErrConversionResultMissing)
		}
		problems = append(problems, "LibreOffice: "+err.Error())
	} else {
		problems = append(problems, userstrings.ErrLibreOfficeMissing)
	}

	if runtime.GOOS == "windows" {
		err := convertWithOfficeCOM(path, converted, target)
		if err == nil {
			if info, statErr := os.Stat(converted); statErr == nil && info.Size() > 0 {
				return converted, nil
			}
			err = errors.New(userstrings.ErrConversionResultMissing)
		}
		problems = append(problems, "Microsoft Office COM: "+err.Error())
	}
	return "", errors.New(strings.Join(problems, "; "))
}

func findSoffice() string {
	for _, name := range []string{"soffice", "soffice.exe", "libreoffice"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	candidates := []string{
		`C:\Program Files\LibreOffice\program\soffice.exe`,
		`C:\Program Files (x86)\LibreOffice\program\soffice.exe`,
		"/usr/bin/soffice",
		"/usr/bin/libreoffice",
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func runConversion(binary string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), conversionTimeout)
	defer cancel()

	output, err := exec.CommandContext(ctx, binary, args...).CombinedOutput()
	if err == nil {
		return nil
	}
	detail := strings.TrimSpace(string(output))
	if len(detail) > 300 {
		detail = detail[:300] + "…"
	}
	if detail == "" {
		return err
	}
	return fmt.Errorf("%w (%s)", err, detail)
}

var officeCOMFormats = map[string]int{".xlsx": 51, ".docx": 16, ".pptx": 24}

func convertWithOfficeCOM(src, dst, target string) error {
	format, ok := officeCOMFormats[target]
	if !ok {
		return fmt.Errorf(userstrings.FmtNoCOMFormat, target)
	}
	source, destination := psQuote(src), psQuote(dst)

	var script string
	switch target {
	case ".xlsx":
		script = fmt.Sprintf(`$ErrorActionPreference='Stop';$app=New-Object -ComObject Excel.Application;$app.Visible=$false;$app.DisplayAlerts=$false;try{$wb=$app.Workbooks.Open(%s,0,$true);$wb.SaveAs(%s,%d);$wb.Close($false)}finally{$app.Quit()}`, source, destination, format)
	case ".docx":
		script = fmt.Sprintf(`$ErrorActionPreference='Stop';$app=New-Object -ComObject Word.Application;$app.Visible=$false;try{$doc=$app.Documents.Open(%s,$false,$true);$doc.SaveAs2(%s,%d);$doc.Close($false)}finally{$app.Quit()}`, source, destination, format)
	default:

		script = fmt.Sprintf(`$ErrorActionPreference='Stop';$app=New-Object -ComObject PowerPoint.Application;try{$pres=$app.Presentations.Open(%s,$true,$false,$false);$pres.SaveAs(%s,%d);$pres.Close()}finally{$app.Quit()}`, source, destination, format)
	}
	return runConversion("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
}

func psQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
