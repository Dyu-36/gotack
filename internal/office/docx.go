package office

import (
	"fmt"
	"regexp"
	"strings"
)

const documentXML = "word/document.xml"

var textElementPattern = regexp.MustCompile(`(<w:t(?:\s[^>]*)?>)(.*?)(</w:t>)`)

func docxParagraphs(documentXML string) ([]string, error) {
	var bodyLines []string

	var (
		inCell   bool
		cellText []string
		rowCells []string
		lineText strings.Builder
	)
	flushLine := func() {
		if lineText.Len() > 0 {
			line := lineText.String()
			if inCell {
				cellText = append(cellText, line)
			} else {
				bodyLines = append(bodyLines, line)
			}
			lineText.Reset()
		}
	}

	err := walkXMLText(documentXML,
		func(name string) {
			switch name {
			case "tc":
				inCell = true
				cellText = nil
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
				rowCells = append(rowCells, strings.Join(cellText, " "))
				inCell = false
				cellText = nil
			case "tr":
				if len(rowCells) > 0 {
					bodyLines = append(bodyLines, strings.Join(rowCells, " | "))
					rowCells = nil
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

func docxCreate(path, content string) error {
	var body strings.Builder
	number := 0
	for _, block := range ParseBlocks(content) {
		switch block.Kind {
		case kindDivider:
			number = 0
			body.WriteString(`<w:p><w:pPr><w:pBdr><w:bottom w:val="single" w:sz="6" w:space="1" w:color="auto"/></w:pBdr></w:pPr></w:p>`)
		case kindHeading:
			number = 0
			body.WriteString(paragraphXML([]Span{{Text: block.Text, Bold: true}}, headingSize(block.Level), true))
		case kindParagraph:
			number = 0
			body.WriteString(paragraphXML(Inline(block.Text), 0, false))
		case kindBullet:
			number = 0
			body.WriteString(paragraphXML(append([]Span{{Text: "•  "}}, Inline(block.Text)...), 0, false))
		case kindNumber:
			number++
			prefix := fmt.Sprintf("%d.  ", number)
			body.WriteString(paragraphXML(append([]Span{{Text: prefix}}, Inline(block.Text)...), 0, false))
		case kindTable:
			number = 0
			body.WriteString(tableXML(block.Rows))
		}
	}

	parts := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
			`</Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>` +
			`</Relationships>`,
		documentXML: `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
			`<w:body>` + body.String() +
			`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1134" w:right="1134" w:bottom="1134" w:left="1134"/></w:sectPr>` +
			`</w:body></w:document>`,
	}
	return writePackage(path, parts)
}

func docxReplace(path, find, replace string) (int, error) {
	if find == "" {
		return 0, fmt.Errorf("office: find text is required")
	}
	raw, err := readPackagePart(path, documentXML)
	if err != nil {
		return 0, err
	}
	needle := escapeXML(find)
	swapped := escapeXML(replace)
	count := strings.Count(raw, needle)
	if count == 0 {
		return 0, fmt.Errorf("office: %q not found in %s (text split across formatting runs is not supported)", find, path)
	}
	updated := textElementPattern.ReplaceAllStringFunc(raw, func(match string) string {
		groups := textElementPattern.FindStringSubmatch(match)
		return groups[1] + strings.ReplaceAll(groups[2], needle, swapped) + groups[3]
	})
	if err := replacePackagePart(path, documentXML, updated); err != nil {
		return 0, err
	}
	return count, nil
}

func headingSize(level int) int {
	switch level {
	case 1:
		return 32
	case 2:
		return 28
	default:
		return 24
	}
}

func paragraphXML(spans []Span, halfPoints int, spaceBefore bool) string {
	var runProperties string
	if halfPoints > 0 {
		runProperties = fmt.Sprintf(`<w:sz w:val="%d"/><w:szCs w:val="%d"/>`, halfPoints, halfPoints)
	}
	paragraphProperties := ""
	if spaceBefore {
		paragraphProperties = `<w:pPr><w:spacing w:before="240" w:after="120"/></w:pPr>`
	} else {
		paragraphProperties = `<w:pPr><w:spacing w:after="120"/></w:pPr>`
	}

	var runs strings.Builder
	for _, span := range spans {
		flags := ""
		if span.Bold {
			flags += "<w:b/>"
		}
		if span.Italic {
			flags += "<w:i/>"
		}
		runs.WriteString(`<w:r><w:rPr>` + flags + runProperties + `</w:rPr><w:t xml:space="preserve">` + escapeXML(span.Text) + `</w:t></w:r>`)
	}
	return `<w:p>` + paragraphProperties + runs.String() + `</w:p>`
}

func tableXML(rows []string) string {
	if len(rows) == 0 {
		return ""
	}
	border := `<w:tblBorders>` +
		`<w:top w:val="single" w:sz="4" w:color="auto"/><w:left w:val="single" w:sz="4" w:color="auto"/>` +
		`<w:bottom w:val="single" w:sz="4" w:color="auto"/><w:right w:val="single" w:sz="4" w:color="auto"/>` +
		`<w:insideH w:val="single" w:sz="4" w:color="auto"/><w:insideV w:val="single" w:sz="4" w:color="auto"/>` +
		`</w:tblBorders>`

	var xml strings.Builder
	xml.WriteString(`<w:tbl><w:tblPr><w:tblW w:w="0" w:type="auto"/>` + border + `</w:tblPr>`)
	for i, row := range rows {
		xml.WriteString(`<w:tr>`)
		for _, cell := range strings.Split(row, "\t") {
			style := ""
			if i == 0 {
				style = `<w:rPr><w:b/></w:rPr>`
			}
			xml.WriteString(`<w:tc><w:tcPr><w:tcW w:w="0" w:type="auto"/></w:tcPr>` +
				`<w:p><w:r>` + style + `<w:t xml:space="preserve">` + escapeXML(strings.TrimSpace(cell)) + `</w:t></w:r></w:p></w:tc>`)
		}
		xml.WriteString(`</w:tr>`)
	}
	xml.WriteString(`</w:tbl>`)

	xml.WriteString(`<w:p/>`)
	return xml.String()
}
