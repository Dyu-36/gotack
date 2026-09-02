// Package skillmanage implements bounded atomic mutations for user-scoped
// procedural skills. Crush owns catalog discovery and ordinary skill reads;
// the writer keeps a small same-process skill_view handshake so autonomous
// reviews may mutate only skills they created and freshly inspected.
package skillmanage
