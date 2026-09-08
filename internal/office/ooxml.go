package office

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

func walkXMLText(raw string, onStart, onText, onEnd func(string)) error {
	decoder := xml.NewDecoder(strings.NewReader(raw))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		switch element := token.(type) {
		case xml.StartElement:
			if element.Name.Local == "t" {
				var text string
				if err := decoder.DecodeElement(&text, &element); err != nil {
					return err
				}
				onText(text)
				continue
			}
			onStart(element.Name.Local)
		case xml.EndElement:
			onEnd(element.Name.Local)
		}
	}
}

func readPackagePart(path, name string) (string, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("office: open %s: %w", path, err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		if file.Name != name {
			continue
		}
		source, err := file.Open()
		if err != nil {
			return "", fmt.Errorf("office: read %s in %s: %w", name, path, err)
		}
		defer source.Close()
		data, err := io.ReadAll(source)
		if err != nil {
			return "", fmt.Errorf("office: read %s in %s: %w", name, path, err)
		}
		return string(data), nil
	}
	return "", fmt.Errorf("office: %s is missing %s", path, name)
}

func listPackageParts(path, prefix string) ([]string, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("office: open %s: %w", path, err)
	}
	defer reader.Close()

	var names []string
	for _, file := range reader.File {
		if strings.HasPrefix(file.Name, prefix) {
			names = append(names, file.Name)
		}
	}
	byTrailingNumber(names)
	return names, nil
}

func byTrailingNumber(names []string) {
	key := func(name string) int {
		digits := 0
		mult := 1
		for i := len(name) - 1; i >= 0 && name[i] >= '0' && name[i] <= '9'; i-- {
			digits += int(name[i]-'0') * mult
			mult *= 10
		}
		return digits
	}
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && key(names[j]) < key(names[j-1]); j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}
}
