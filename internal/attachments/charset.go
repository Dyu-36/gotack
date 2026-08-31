package attachments

import (
	"bytes"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/Dyu-36/gotack/internal/userstrings"
)

// charset.go -- role: turn raw attachment bytes into UTF-8 text.
//
// Office exports on Vietnamese Windows are routinely UTF-16 (with BOM) or
// single-byte Windows-1252/1258 text. utf8.Valid rejects those, so before this
// decoder a CSV exported from Excel was classified as binary and the model only
// ever received a file path instead of its content.

// DecodeText converts content into UTF-8 text and names the source encoding.
func DecodeText(content []byte) (string, string) {
	switch {
	case bytes.HasPrefix(content, []byte{0xEF, 0xBB, 0xBF}):
		return string(bytes.TrimPrefix(content, []byte{0xEF, 0xBB, 0xBF})), "UTF-8 (BOM)"
	case bytes.HasPrefix(content, []byte{0xFF, 0xFE}):
		return decodeUTF16(content[2:], false), "UTF-16LE"
	case bytes.HasPrefix(content, []byte{0xFE, 0xFF}):
		return decodeUTF16(content[2:], true), "UTF-16BE"
	}
	if utf8.Valid(content) && bytes.IndexByte(content, 0x00) < 0 {
		return string(content), "UTF-8"
	}
	if little, ok := looksLikeUTF16(content); ok {
		return decodeUTF16(content, !little), userstrings.EncodingUTF16NoBOM
	}
	return decodeCP1252(content), "Windows-1252"
}

func decodeUTF16(content []byte, bigEndian bool) string {
	if len(content)%2 == 1 {
		content = content[:len(content)-1]
	}
	units := make([]uint16, 0, len(content)/2)
	for i := 0; i+1 < len(content); i += 2 {
		if bigEndian {
			units = append(units, uint16(content[i])<<8|uint16(content[i+1]))
			continue
		}
		units = append(units, uint16(content[i+1])<<8|uint16(content[i]))
	}
	return strings.ReplaceAll(string(utf16.Decode(units)), "\x00", "")
}

// looksLikeUTF16 detects BOM-less UTF-16 through the NUL padding byte that
// every ASCII character carries: little-endian text pads odd offsets,
// big-endian text pads even offsets.
func looksLikeUTF16(content []byte) (little bool, ok bool) {
	limit := min(2048, len(content))
	if limit < 4 {
		return false, false
	}
	var even, odd int
	for i := 0; i < limit; i++ {
		if content[i] != 0x00 {
			continue
		}
		if i%2 == 0 {
			even++
			continue
		}
		odd++
	}
	pairs := limit / 2
	switch {
	case odd*4 > pairs*3:
		return true, true
	case even*4 > pairs*3:
		return false, true
	}
	return false, false
}

// cp1252High maps the 0x80..0x9F range where Windows-1252 differs from
// ISO-8859-1. A zero entry marks an undefined byte, which is dropped.
var cp1252High = [32]rune{
	'\u20AC', 0, '\u201A', '\u0192', '\u201E', '\u2026', '\u2020', '\u2021',
	'\u02C6', '\u2030', '\u0160', '\u2039', '\u0152', 0, '\u017D', 0,
	0, '\u2018', '\u2019', '\u201C', '\u201D', '\u2022', '\u2013', '\u2014',
	'\u02DC', '\u2122', '\u0161', '\u203A', '\u0153', 0, '\u017E', '\u0178',
}

func decodeCP1252(content []byte) string {
	var sb strings.Builder
	sb.Grow(len(content))
	for _, b := range content {
		switch {
		case b == 0x00:
		case b < 0x80:
			sb.WriteByte(b)
		case b < 0xA0:
			if r := cp1252High[b-0x80]; r != 0 {
				sb.WriteRune(r)
			}
		default:
			sb.WriteRune(rune(b))
		}
	}
	return sb.String()
}
