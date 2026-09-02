// memory is the gotack persistent-memory MCP server.
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
)

func main() {
	dir := flag.String("dir", "", "memory directory (default: <appconfig dir>/context/memory)")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server := &mcp.Server{
		Name:    serverName,
		Version: serverVersion,
		Tools:   []mcp.Tool{memory.Tool(memory.NewStore(resolveDir(*dir)))},
	}
	if err := server.Serve(ctx, os.Stdin, os.Stdout); err != nil && ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "memory: %v\n", err)
		os.Exit(1)
	}
}

func resolveDir(dirFlag string) string {
	if dirFlag != "" {
		return dirFlag
	}
	return filepath.Join(appconfig.Dir(), "context", "memory")
}
