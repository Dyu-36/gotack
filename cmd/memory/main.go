// memory is the gotack persistent-memory MCP server. It exposes the single
// memory tool that curates MEMORY.md and USER.md under the seeded context
// directory, following the cmd/office + internal/mcp stdio pattern. Host
// registration is a one-line mcp_servers entry; see
// docs/contracts/gotack-memory-mcp.md.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/mcp"
	"github.com/Dyu-36/gotack/internal/memory"
)

const (
	serverName    = "gotack-memory"
	serverVersion = "0.1.0"

	// sessionEnvOverride lets the host or a test pin the provenance session.
	// sessionEnvEngine is what the engine would export if it ever attached
	// the session id to MCP servers (today it only exports it to hooks, so
	// the fallback below is the common case).
	sessionEnvOverride = "GOTACK_MEMORY_SESSION"
	sessionEnvEngine   = "CRUSH_SESSION_ID"
	// sessionUnknown labels provenance when no session id is available, so
	// every entry still carries a traceable writer field.
	sessionUnknown = "unknown"
)

func main() {
	dir := flag.String("dir", "", "memory directory (default: <appconfig dir>/context/memory)")
	session := flag.String("session", "", "writer recorded in provenance (default: $"+sessionEnvOverride+" or $"+sessionEnvEngine+` or "`+sessionUnknown+`")`)
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store := memory.NewStore(resolveDir(*dir), resolveSession(*session))
	server := &mcp.Server{
		Name:    serverName,
		Version: serverVersion,
		Tools:   []mcp.Tool{memory.Tool(store)},
	}
	if err := server.Serve(ctx, os.Stdin, os.Stdout); err != nil && ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "memory: %v\n", err)
		os.Exit(1)
	}
}

// resolveDir applies flag > gotack default. The default sits inside the
// seeded context directory so Crush injects the files automatically, and
// outside any workspace so the guard's write-safe root cannot cover it.
func resolveDir(dirFlag string) string {
	if dirFlag != "" {
		return dirFlag
	}
	return filepath.Join(appconfig.Dir(), "context", "memory")
}

// resolveSession applies flag > override env > engine env > "unknown".
func resolveSession(sessionFlag string) string {
	if sessionFlag != "" {
		return sessionFlag
	}
	if value := os.Getenv(sessionEnvOverride); value != "" {
		return value
	}
	if value := os.Getenv(sessionEnvEngine); value != "" {
		return value
	}
	return sessionUnknown
}
