package office

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

func xlsxInfo(path string) (string, error) {
	file, err := excelize.OpenFile(path)
	if err != nil {
		return "", fmt.Errorf("office: open %s: %w", path, err)
	}
	defer file.Close()

	sheets := file.GetSheetList()
	if len(sheets) == 0 {
		return "Excel workbook: no sheets", nil
	}
	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Excel workbook: %d sheets", len(sheets)))
	for _, sheet := range sheets {
		rows, err := file.GetRows(sheet)
		if err != nil {
			continue
		}
		cols := 0
		for _, row := range rows {
			if len(row) > cols {
				cols = len(row)
			}
		}
		fmt.Fprintf(&summary, "; %q %dx%d", sheet, len(rows), cols)
	}
	return summary.String(), nil
}

func xlsxRead(path, sheet string, maxChars int) (string, error) {
	file, err := excelize.OpenFile(path)
	if err != nil {
		return "", fmt.Errorf("office: open %s: %w", path, err)
	}
	defer file.Close()

	if sheet == "" {
		sheets := file.GetSheetList()
		if len(sheets) == 0 {
			return "", fmt.Errorf("office: %s has no sheets", path)
		}
		sheet = sheets[0]
	}
	rows, err := file.GetRows(sheet)
	if err != nil {
		return "", fmt.Errorf("office: read sheet %q: %w", sheet, err)
	}

	var out strings.Builder
	for _, row := range rows {
		line := strings.Join(row, "\t")
		if out.Len()+len(line) > maxChars {
			out.WriteString("…(output truncated)")
			break
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return strings.TrimRight(out.String(), "\n"), nil
}
