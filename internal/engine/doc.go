// Package engine owns the lifecycle of the Crush server process.
//
// Responsibilities: locate a reachable server, launch one when none exists,
// supervise it, and stop only what this host started. A UI restart must never
// kill a running agent.
//
// This package does not speak HTTP, see internal/crushapi.
package engine
