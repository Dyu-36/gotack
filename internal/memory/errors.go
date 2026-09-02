package memory

import (
	"errors"
	"fmt"
)

var (
	ErrUnknownAction  = errors.New(`memory: unknown action (want "add", "replace" or "remove")`)
	ErrUnknownTarget  = errors.New(`memory: unknown target (want "memory" or "user")`)
	ErrEmptyContent   = errors.New("memory: content must not be empty")
	ErrMissingOldText = errors.New("memory: old_text is required for replace and remove")
	ErrTextNotFound   = errors.New("memory: old_text not found")
	ErrTextNotUnique  = errors.New("memory: old_text is not unique")
	ErrOverCap        = errors.New("memory: write refused, file is at its cap")
	ErrBlocked        = errors.New("memory: content blocked")
	ErrArguments      = errors.New("memory: invalid arguments")
	ErrInvalidUTF8    = errors.New("memory: file is not valid UTF-8")
	ErrEmptyBatch     = errors.New("memory: operations list is empty")
)

type OverCapError struct {
	Target  Target
	Used    int
	Cap     int
	Wanted  int
	Entries []string
}

func (e *OverCapError) Error() string {
	return fmt.Sprintf(
		"Memory at %s/%s chars. Proposed final state is %s chars. Consolidate in one batch: remove or shorten entries, then add the new entry; nothing was changed.",
		group(e.Used), group(e.Cap), group(e.Wanted),
	)
}

func (e *OverCapError) Unwrap() error { return ErrOverCap }

type operationError struct {
	cause   error
	entries []string
	used    int
	cap     int
}

func (e *operationError) Error() string { return e.cause.Error() }
func (e *operationError) Unwrap() error { return e.cause }
