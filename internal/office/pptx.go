package office

import (
	"fmt"
	"strings"
)

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
		if err := walkXMLText(raw,
			func(name string) {
				if name == "p" {
					current.Reset()
				}
			},
			func(text string) { current.WriteString(text) },
			func(name string) {
				if name == "p" && current.Len() > 0 {
					slideTexts = append(slideTexts, current.String())
					current.Reset()
				}
			},
		); err != nil {
			return nil, fmt.Errorf("office: parse %s in %s: %w", name, path, err)
		}
		slides = append(slides, strings.Join(slideTexts, "\n"))
	}
	return slides, nil
}

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
