package attachments

import (
	"encoding/base64"
	"fmt"

	"github.com/Dyu-36/gotack/internal/userstrings"
)

type Input struct {
	FileName string
	MimeType string
	Content  string
	Path     string
}

func PrepareInputs(input []Input, supportsVision bool) []Prepared {
	out := make([]Prepared, 0, len(input))
	for i, item := range input {
		name := BaseName(item.FileName)
		if name == "" {
			name = fmt.Sprintf("attachment-%d.bin", i+1)
		}

		if item.Path != "" {
			prepared, err := PrepareFile(item.Path, supportsVision)
			if err != nil {
				out = append(out, Failed(name, err.Error()))
				continue
			}
			out = append(out, prepared)
			continue
		}
		if len(item.Content) > base64.StdEncoding.EncodedLen(MaxAttachmentSize) {
			out = append(out, Failed(name, userstrings.AttachmentTooLarge))
			continue
		}
		content, err := base64.StdEncoding.DecodeString(item.Content)
		if err != nil {
			out = append(out, Failed(name, userstrings.AttachmentInvalidUpload))
			continue
		}
		if len(content) > MaxAttachmentSize {
			out = append(out, Failed(name, userstrings.AttachmentTooLarge))
			continue
		}
		prepared, err := Prepare(name, item.MimeType, content, supportsVision)
		if err != nil {
			out = append(out, Failed(name, err.Error()))
			continue
		}
		out = append(out, prepared)
	}
	return out
}
