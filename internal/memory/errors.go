// Package memory implements gotack's persistent self-editing memory: the
// dedicated MCP tool that curates MEMORY.md and USER.md inside the seeded
// context directory (<appconfig.Dir()>/context/memory). Crush re-reads every
// context file on each prompt construction, so a write here lands in the
// system prompt on the next turn with no engine restart.
//
// Per docs/decisions/0003-memory-writes-constrained-by-construction.md this
// tool is the only sanctioned writer of the memory files: writes carry hard
// size caps with oldest-first eviction, provenance on every entry, and
// atomic (temp file plus rename) persistence. The Phase 4 guard denies every
// other write path into the context directory (rule "memory-context-write"
// in internal/guard), so the caps cannot be bypassed through a side door.
package memory

import "errors"

// ErrUnknownAction reports an action outside view/add/replace/remove. The
// tool rejects unknown actions with a typed error instead of guessing, per
// decision 0003: the writer must be constrained by construction.
var ErrUnknownAction = errors.New("memory: unknown action (want view, add, replace or remove)")

// ErrUnknownTarget reports a target outside memory/user.
var ErrUnknownTarget = errors.New(`memory: unknown target (want "memory" or "user")`)

// ErrEmptyContent reports an add or replace whose content is empty or
// whitespace-only; storing it would burn provenance on nothing.
var ErrEmptyContent = errors.New("memory: content must not be empty for add or replace")

// ErrMissingSection reports an add, replace or remove without a section
// heading; sections are the unit of editing, so the heading is required.
var ErrMissingSection = errors.New("memory: section heading is required for add, replace and remove")

// ErrSectionNotFound reports a replace or remove against a section that does
// not exist. These are explicit errors rather than silent no-ops so the
// model learns the section name it used is wrong.
var ErrSectionNotFound = errors.New("memory: section not found")

// ErrCapExceeded reports a write whose entry alone exceeds the file cap
// even after every other entry was evicted. The error names the cap so the
// model consolidates instead of failing silently.
var ErrCapExceeded = errors.New("memory: entry exceeds the file size cap")

// ErrArguments reports absent or malformed tool arguments.
var ErrArguments = errors.New("memory: invalid arguments")
