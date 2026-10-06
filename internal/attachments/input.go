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
		out = append(out, prepareInput(item, i, supportsVision))
	}
	return out
}

func prepareInput(item Input, index int, supportsVision bool) Prepared {
	name := BaseName(item.FileName)
	if name == "" {
		name = fmt.Sprintf("attachment-%d.bin", index+1)
	}
	if item.Path != "" {
		return prepareFileInput(name, item.Path, supportsVision)
	}
	return prepareEncodedInput(name, item, supportsVision)
}

func prepareFileInput(name, path string, supportsVision bool) Prepared {
	prepared, err := PrepareFile(path, supportsVision)
	if err != nil {
		return Failed(name, err.Error())
	}
	return prepared
}

func prepareEncodedInput(name string, item Input, supportsVision bool) Prepared {
	if len(item.Content) > base64.StdEncoding.EncodedLen(MaxAttachmentSize) {
		return Failed(name, userstrings.AttachmentTooLarge)
	}
	content, err := base64.StdEncoding.DecodeString(item.Content)
	if err != nil {
		return Failed(name, userstrings.AttachmentInvalidUpload)
	}
	if len(content) > MaxAttachmentSize {
		return Failed(name, userstrings.AttachmentTooLarge)
	}
	prepared, err := Prepare(name, item.MimeType, content, supportsVision)
	if err != nil {
		return Failed(name, err.Error())
	}
	return prepared
}
