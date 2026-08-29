package office

import (
	"strconv"
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
)

// pptx.go -- role: read, create and text-edit PowerPoint .pptx packages. The
// writer emits a minimal valid package (master, layout, theme, one text-box
// shape per content block) so generated decks open without a template file.

var drawingTextPattern = regexp.MustCompile(`(<a:t(?:\s[^>]*)?>)(.*?)(</a:t>)`)

type pptxPart struct {
	Name    string
	Content string
}

// pptxSlideTexts returns the concatenated shape text of each slide, slides in
// presentation order.
func pptxSlideTexts(path string) ([]string, error) {
	names, err := listPackageParts(path, "ppt/slides/slide")
	if err != nil {
		return nil, err
	}
	var slides []string
	for _, name := range names {
		raw, err := readPackagePart(path, name)
		if err != nil {
			return nil, err
		}
		var slideTexts []string
		var current strings.Builder
		decoder := xml.NewDecoder(strings.NewReader(raw))
		for {
			token, err := decoder.Token()
			if err != nil {
				break
			}
			switch element := token.(type) {
			case xml.StartElement:
				switch element.Name.Local {
				case "p":
					current.Reset()
				case "t":
					var text string
					if err := decoder.DecodeElement(&text, &element); err == nil {
						current.WriteString(text)
					}
				}
			case xml.EndElement:
				if element.Name.Local == "p" && current.Len() > 0 {
					slideTexts = append(slideTexts, current.String())
					current.Reset()
				}
			}
		}
		slides = append(slides, strings.Join(slideTexts, "\n"))
	}
	return slides, nil
}

// pptxRead renders the deck as "## Slide N" blocks with one line per shape
// paragraph, mirroring the markdown source format.
func pptxRead(path string) (string, error) {
	slides, err := pptxSlideTexts(path)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	for i, slide := range slides {
		fmt.Fprintf(&out, "## Slide %d\n%s\n", i+1, slide)
	}
	return strings.TrimSpace(out.String()), nil
}

// pptxInfo summarizes the deck structure.
func pptxInfo(path string) (string, error) {
	slides, err := pptxSlideTexts(path)
	if err != nil {
		return "", err
	}
	paragraphs := 0
	for _, slide := range slides {
		paragraphs += strings.Count(slide, "\n") + 1
	}
	return fmt.Sprintf("PowerPoint presentation: %d slides, %d text paragraphs", len(slides), paragraphs), nil
}

// pptxCreate builds a .pptx from the markdown subset. Each `# Heading` starts
// a slide (its text becomes the title); bullets, paragraphs and tables below
// it fill the slide body; --- forces a new untitled slide.
func pptxCreate(path, content string) error {
	type slide struct {
		title  string
		blocks []string
	}
	var slides []*slide
	current := &slide{}
	closeSlide := func() {
		if current.title != "" || len(current.blocks) > 0 {
			slides = append(slides, current)
			current = &slide{}
		}
	}
	for _, block := range ParseBlocks(content) {
		switch block.Kind {
		case kindHeading:
			if block.Level == 1 {
				closeSlide()
				current.title = block.Text
			} else {
				current.blocks = append(current.blocks, block.Text)
			}
		case kindParagraph, kindBullet, kindNumber:
			text := block.Text
			if block.Kind == kindBullet {
				text = "•  " + text
			}
			if block.Kind == kindNumber {
				text = "-  " + text
			}
			current.blocks = append(current.blocks, text)
		case kindTable:
			for _, row := range block.Rows {
				current.blocks = append(current.blocks, strings.ReplaceAll(row, "\t", "   "))
			}
		case kindDivider:
			closeSlide()
		}
	}
	closeSlide()
	if len(slides) == 0 {
		return fmt.Errorf("office: pptx content must contain at least one '# Slide title'")
	}

	parts := pptxSkeleton(len(slides))
	for i, slide := range slides {
		parts["ppt/slides/slide"+strconv.Itoa(i+1)+".xml"] = slideXML(slide.title, slide.blocks)
		parts["ppt/slides/_rels/slide"+strconv.Itoa(i+1)+".xml.rels"] = slideRelsXML()
		parts["[Content_Types].xml"] = strings.Replace(
			parts["[Content_Types].xml"], "</Types>",
			`<Override PartName="/ppt/slides/slide`+strconv.Itoa(i+1)+`.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/></Types>`, 1)
	}
	return writePackage(path, parts)
}

// pptxReplace replaces find with replace inside slide text nodes.
func pptxReplace(path, find, replace string) (int, error) {
	if find == "" {
		return 0, fmt.Errorf("office: find text is required")
	}
	names, err := listPackageParts(path, "ppt/slides/slide")
	if err != nil {
		return 0, err
	}
	needle := escapeXML(find)
	swapped := escapeXML(replace)
	total := 0
	for _, name := range names {
		raw, err := readPackagePart(path, name)
		if err != nil {
			return total, err
		}
		count := strings.Count(raw, needle)
		if count == 0 {
			continue
		}
		updated := drawingTextPattern.ReplaceAllStringFunc(raw, func(match string) string {
			groups := drawingTextPattern.FindStringSubmatch(match)
			return groups[1] + strings.ReplaceAll(groups[2], needle, swapped) + groups[3]
		})
		if err := replacePackagePart(path, name, updated); err != nil {
			return total, err
		}
		total += count
	}
	if total == 0 {
		return 0, fmt.Errorf("office: %q not found in %s", find, path)
	}
	return total, nil
}

func slideXML(title string, blocks []string) string {
	nextID := 2
	shapes := textboxXML(nextID, 457200, 274638, 8229600, 914400, escapeXML(title), 3200, true)
	for i, block := range blocks {
		// Stack body text boxes below the title; roughly 1.3 lines per block.
		nextID++
		y := 1188720 + int64(i)*1362075
		shapes += textboxXML(nextID, 457200, y, 8229600, 1270000, escapeXML(block), 1600, false)
	}
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
		`<p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
		`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>` +
		shapes + `</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>`
}

func textboxXML(id int, x, y, w, h int64, text string, size int, bold bool) string {
	style := ""
	if bold {
		style = `b="1" `
	}
	return `<p:sp><p:nvSpPr><p:cNvPr id="` + strconv.Itoa(id) + `" name="TextBox` + strconv.Itoa(id) + `"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr>` +
		`<p:spPr><a:xfrm><a:off x="` + strconv.FormatInt(x, 10) + `" y="` + strconv.FormatInt(y, 10) + `"/><a:ext cx="` + strconv.FormatInt(w, 10) + `" cy="` + strconv.FormatInt(h, 10) + `"/></a:xfrm>` +
		`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>` +
		`<p:txBody><a:bodyPr wrap="square"/><a:lstStyle/><a:p><a:r><a:rPr lang="en-US" ` + style + `sz="` + strconv.Itoa(size) + `"/>` +
		`<a:t>` + text + `</a:t></a:r></a:p></p:txBody></p:sp>`
}

func slideRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>` +
		`</Relationships>`
}

// pptxSkeleton returns the fixed parts of a minimal deck; slide overrides are
// appended into [Content_Types].xml by pptxCreate.
func pptxSkeleton(slideCount int) map[string]string {
	slideIDs := strings.Builder{}
	for i := 1; i <= slideCount; i++ {
		fmt.Fprintf(&slideIDs, `<p:sldId id="%d" r:id="rId%d"/>`, 256+i, i+1)
	}
	relationships := strings.Builder{}
	for i := 1; i <= slideCount; i++ {
		fmt.Fprintf(&relationships, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide%d.xml"/>`, i+1, i)
	}

	return map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>` +
			`<Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>` +
			`<Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>` +
			`<Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>` +
			`</Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>` +
			`</Relationships>`,
		"ppt/presentation.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
			`<p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst>` +
			`<p:sldIdLst>` + slideIDs.String() + `</p:sldIdLst>` +
			`<p:sldSz cx="9144000" cy="6858000"/><p:notesSz cx="6858000" cy="9144000"/></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/>` +
			relationships.String() +
			`<Relationship Id="rId` + strconv.Itoa(slideCount+2) + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="theme/theme1.xml"/>` +
			`</Relationships>`,
		"ppt/slideMasters/slideMaster1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<p:sldMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
			`<p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld>` +
			`<p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>` +
			`<p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rId1"/></p:sldLayoutIdLst></p:sldMaster>`,
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>` +
			`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/>` +
			`</Relationships>`,
		"ppt/slideLayouts/slideLayout1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" type="blank" preserve="1">` +
			`<p:cSld name="Blank"><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld>` +
			`<p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sldLayout>`,
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/>` +
			`</Relationships>`,
		"ppt/theme/theme1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Gotack">` +
			`<a:themeElements><a:clrScheme name="Gotack"><a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1>` +
			`<a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1><a:dk2><a:srgbClr val="1F2933"/></a:dk2><a:lt2><a:srgbClr val="F5F7FA"/></a:lt2>` +
			`<a:accent1><a:srgbClr val="2563EB"/></a:accent1><a:accent2><a:srgbClr val="0EA5E9"/></a:accent2><a:accent3><a:srgbClr val="10B981"/></a:accent3>` +
			`<a:accent4><a:srgbClr val="F59E0B"/></a:accent4><a:accent5><a:srgbClr val="8B5CF6"/></a:accent5><a:accent6><a:srgbClr val="EF4444"/></a:accent6>` +
			`<a:hlink><a:srgbClr val="2563EB"/></a:hlink><a:folHlink><a:srgbClr val="6366F1"/></a:folHlink></a:clrScheme>` +
			`<a:fontScheme name="Gotack"><a:majorFont><a:latin typeface="Calibri Light"/><a:ea typeface=""/><a:cs typeface=""/></a:majorFont>` +
			`<a:minorFont><a:latin typeface="Calibri"/><a:ea typeface=""/><a:cs typeface=""/></a:minorFont></a:fontScheme>` +
			`<a:fmtScheme name="Gotack"><a:fillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill>` +
			`<a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:fillStyleLst>` +
			`<a:lnStyleLst><a:ln w="6350"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln>` +
			`<a:ln w="12700"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln>` +
			`<a:ln w="19050"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln></a:lnStyleLst>` +
			`<a:effectStyleLst><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle>` +
			`<a:effectStyle><a:effectLst/></a:effectStyle></a:effectStyleLst><a:bgFillStyleLst>` +
			`<a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill>` +
			`<a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:bgFillStyleLst></a:fmtScheme></a:themeElements></a:theme>`,
	}
}


