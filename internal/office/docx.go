package office

import (
	"fmt"
	"strings"
)

const documentXML = "word/document.xml"

func docxParagraphs(documentXML string) ([]string, error) {
	var bodyLines []string

	var (
		inCell      bool
		cellText    []string
		rowCells    []string
		inCellStack []bool
		cellStack   [][]string
		rowStack    [][]string
		lineText    strings.Builder
	)
	flushLine := func() {
		if lineText.Len() == 0 {
			return
		}
		line := lineText.String()
		if inCell {
			cellText = append(cellText, line)
		} else {
			bodyLines = append(bodyLines, line)
		}
		lineText.Reset()
	}

	err := walkXMLText(documentXML,
		func(name string) {
			switch name {
			case "tc":
				inCellStack = append(inCellStack, inCell)
				cellStack = append(cellStack, cellText)
				rowStack = append(rowStack, rowCells)
				inCell = true
				cellText = nil
				rowCells = nil
			case "br":
				lineText.WriteString(" ")
			}
		},
		func(text string) { lineText.WriteString(text) },
		func(name string) {
			switch name {
			case "p":
				flushLine()
			case "tc":
				flushLine()
				cell := strings.Join(cellText, " ")
				top := len(cellStack) - 1
				inCell = inCellStack[top]
				cellText = cellStack[top]
				rowCells = rowStack[top]
				inCellStack = inCellStack[:top]
				cellStack = cellStack[:top]
				rowStack = rowStack[:top]
				rowCells = append(rowCells, cell)
			case "tr":
				if len(rowCells) > 0 {
					row := strings.Join(rowCells, " | ")
					rowCells = nil
					if inCell {
						cellText = append(cellText, row)
					} else {
						bodyLines = append(bodyLines, row)
					}
				}
			}
		},
	)
	if err != nil {
		return nil, err
	}
	return bodyLines, nil
}

func docxRead(path string) (string, error) {
	raw, err := readPackagePart(path, documentXML)
	if err != nil {
		return "", err
	}
	lines, err := docxParagraphs(raw)
	if err != nil {
		return "", fmt.Errorf("office: parse %s: %w", path, err)
	}
	return strings.Join(lines, "\n"), nil
}

func docxInfo(path string) (string, error) {
	raw, err := readPackagePart(path, documentXML)
	if err != nil {
		return "", err
	}
	lines, err := docxParagraphs(raw)
	if err != nil {
		return "", fmt.Errorf("office: parse %s: %w", path, err)
	}
	tableRows := 0
	for _, line := range lines {
		if strings.Contains(line, " | ") {
			tableRows++
		}
	}
	return fmt.Sprintf("Word document: %d paragraphs (%d table rows), %d characters", len(lines)-tableRows, tableRows, len(strings.Join(lines, ""))), nil
}
