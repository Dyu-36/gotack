package office

// ooxml.go -- role: zip-package plumbing shared by the Word and PowerPoint
// writers and readers. OOXML files are zip archives of XML parts.

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"strings"
)

// writePackage serializes parts (name -> XML content) into an OOXML zip file.
func writePackage(path string, parts map[string]string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("office: create %s: %w", path, err)
	}
	defer file.Close()

	zipper := zip.NewWriter(file)
	// Deterministic order: [Content_Types].xml first, then sorted names.
	if _, ok := parts["[Content_Types].xml"]; ok {
		if err := writePart(zipper, "[Content_Types].xml", parts["[Content_Types].xml"]); err != nil {
			zipper.Close()
			return err
		}
	}
	for _, name := range sortedOtherParts(parts) {
		if err := writePart(zipper, name, parts[name]); err != nil {
			zipper.Close()
			return err
		}
	}
	if err := zipper.Close(); err != nil {
		return fmt.Errorf("office: pack %s: %w", path, err)
	}
	return file.Close()
}

func sortedOtherParts(parts map[string]string) []string {
	names := make([]string, 0, len(parts))
	for name := range parts {
		if name != "[Content_Types].xml" {
			names = append(names, name)
		}
	}
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j] < names[j-1]; j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}
	return names
}

func writePart(zipper *zip.Writer, name, content string) error {
	entry, err := zipper.Create(name)
	if err != nil {
		return fmt.Errorf("office: pack %s: %w", name, err)
	}
	if _, err := io.WriteString(entry, content); err != nil {
		return fmt.Errorf("office: pack %s: %w", name, err)
	}
	return nil
}

// readPackagePart extracts one XML part from an OOXML file.
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

// replacePackagePart rewrites one part inside an OOXML package while keeping
// every other member byte-for-byte.
func replacePackagePart(path, name, content string) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("office: open %s: %w", path, err)
	}

	temp := path + ".tmp"
	writer, err := os.Create(temp)
	if err != nil {
		reader.Close()
		return fmt.Errorf("office: rewrite %s: %w", path, err)
	}

	zipper := zip.NewWriter(writer)
	replaced := false
	for _, file := range reader.File {
		entry, entryErr := zipper.CreateHeader(&zip.FileHeader{Name: file.Name, Method: file.Method})
		if entryErr != nil {
			zipper.Close()
			writer.Close()
			reader.Close()
			return fmt.Errorf("office: rewrite %s: %w", path, entryErr)
		}
		var copyErr error
		if file.Name == name {
			replaced = true
			_, copyErr = io.Copy(entry, strings.NewReader(content))
		} else {
			source, openErr := file.Open()
			if openErr != nil {
				zipper.Close()
				writer.Close()
				reader.Close()
				return fmt.Errorf("office: rewrite %s: %w", path, openErr)
			}
			_, copyErr = io.Copy(entry, source)
			source.Close()
		}
		if copyErr != nil {
			zipper.Close()
			writer.Close()
			reader.Close()
			return fmt.Errorf("office: rewrite %s: %w", path, copyErr)
		}
	}
	if err := zipper.Close(); err != nil {
		writer.Close()
		reader.Close()
		return fmt.Errorf("office: rewrite %s: %w", path, err)
	}
	if err := writer.Close(); err != nil {
		reader.Close()
		return fmt.Errorf("office: rewrite %s: %w", path, err)
	}
	reader.Close()
	if !replaced {
		_ = os.Remove(temp)
		return fmt.Errorf("office: %s is missing %s", path, name)
	}
	return os.Rename(temp, path)
}

// listPackageParts returns member names matching a prefix, sorted by the
// trailing number so slides keep document order.
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

// byTrailingNumber sorts "base1", "base10", "base2" into numeric order.
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

// escapeXML makes text safe for element and attribute content.
func escapeXML(text string) string {
	var out strings.Builder
	for _, r := range text {
		switch r {
		case '&':
			out.WriteString("&amp;")
		case '<':
			out.WriteString("&lt;")
		case '>':
			out.WriteString("&gt;")
		case '"':
			out.WriteString("&quot;")
		case '\'':
			out.WriteString("&apos;")
		default:
			out.WriteRune(r)
		}
	}
	return out.String()
}
