// Package recall implements cross-session recall over the Crush engine's
// SQLite database. It opens crush.db strictly read-only, extracts searchable
// text from message parts, and maintains its own FTS5 index in recall.db so
// nothing is ever written into the engine-managed, goose-migrated database.
package recall

import "errors"

// ErrSchemaMismatch reports that crush.db no longer matches the schema this
// package depends on (a table or required column is missing). It must surface
// to the caller instead of producing silently empty search results.
var ErrSchemaMismatch = errors.New("recall: crush.db schema does not match the recall contract")

// ErrSourceMissing reports that crush.db does not exist in the data
// directory, typically because the engine has never run there.
var ErrSourceMissing = errors.New("recall: crush.db not found in data directory")

// ErrInvalidQuery reports a search query with no searchable words.
var ErrInvalidQuery = errors.New("recall: search query contains no searchable words")
