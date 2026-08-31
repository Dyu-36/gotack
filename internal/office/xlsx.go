package office

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// xlsx.go -- role: read, create and edit Excel workbooks through excelize.
// office_read emits TSV so the agent can round-trip data without quoting
// ambiguity; office_edit supports set-cell and append-rows operations.

// xlsxInfo summarizes the workbook structure.
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

// xlsxRead dumps one sheet (or the first sheet when empty) as TSV.
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
		out.WriteString("\n")
	}
	return strings.TrimRight(out.String(), "\n"), nil
}

// xlsxCreate builds a workbook whose first sheet holds the TSV content.
func xlsxCreate(path, content string) error {
	file := excelize.NewFile()
	defer file.Close()

	for i, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		if line == "" {
			continue
		}
		row := i + 1
		for j, cell := range strings.Split(line, "\t") {
			cellName, err := excelize.CoordinatesToCellName(j+1, row)
			if err != nil {
				return fmt.Errorf("office: address row %d col %d: %w", row, j+1, err)
			}
			if err := file.SetCellValue("Sheet1", cellName, coerceCell(cell)); err != nil {
				return fmt.Errorf("office: set %s: %w", cellName, err)
			}
		}
	}
	if err := file.SaveAs(path); err != nil {
		return fmt.Errorf("office: save %s: %w", path, err)
	}
	return nil
}

// xlsxSetCell writes one coerced value into a cell.
func xlsxSetCell(path, sheet, cell, value string) error {
	file, err := excelize.OpenFile(path)
	if err != nil {
		return fmt.Errorf("office: open %s: %w", path, err)
	}
	defer file.Close()

	if sheet == "" {
		sheets := file.GetSheetList()
		if len(sheets) == 0 {
			return fmt.Errorf("office: %s has no sheets", path)
		}
		sheet = sheets[0]
	}
	if err := file.SetCellValue(sheet, cell, coerceCell(value)); err != nil {
		return fmt.Errorf("office: set %s!%s: %w", sheet, cell, err)
	}
	if err := file.Save(); err != nil {
		return fmt.Errorf("office: save %s: %w", path, err)
	}
	return nil
}

// xlsxAppendRows appends TSV rows (one row per line) after the last row.
func xlsxAppendRows(path, sheet, tsv string) (int, error) {
	file, err := excelize.OpenFile(path)
	if err != nil {
		return 0, fmt.Errorf("office: open %s: %w", path, err)
	}
	defer file.Close()

	if sheet == "" {
		sheets := file.GetSheetList()
		if len(sheets) == 0 {
			return 0, fmt.Errorf("office: %s has no sheets", path)
		}
		sheet = sheets[0]
	}
	rows, err := file.GetRows(sheet)
	if err != nil {
		return 0, fmt.Errorf("office: read sheet %q: %w", sheet, err)
	}

	appended := 0
	for _, line := range strings.Split(strings.ReplaceAll(tsv, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		appended++
		targetRow := len(rows) + appended
		for j, cell := range strings.Split(line, "\t") {
			cellName, err := excelize.CoordinatesToCellName(j+1, targetRow)
			if err != nil {
				return appended, fmt.Errorf("office: address row %d col %d: %w", targetRow, j+1, err)
			}
			if err := file.SetCellValue(sheet, cellName, coerceCell(cell)); err != nil {
				return appended, fmt.Errorf("office: set %s: %w", cellName, err)
			}
		}
	}
	if appended == 0 {
		return 0, fmt.Errorf("office: no rows to append")
	}
	if err := file.Save(); err != nil {
		return appended, fmt.Errorf("office: save %s: %w", path, err)
	}
	return appended, nil
}

// coerceCell converts TSV text into a typed cell value: numbers and booleans
// become native, everything else stays text.
func coerceCell(value string) any {
	if value == "" {
		return ""
	}
	if number, err := strconv.ParseFloat(value, 64); err == nil {
		return number
	}
	switch strings.ToLower(value) {
	case "true":
		return true
	case "false":
		return false
	default:
		return value
	}
}
