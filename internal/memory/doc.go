// Package memory implements Hermes-compatible bounded MEMORY.md and USER.md
// mutation for the desktop agent. Entries are raw text separated by "\n§\n";
// replace and remove select one whole entry by a unique substring. Mutations
// are interprocess-locked, atomically persisted, and never evict content.
package memory
