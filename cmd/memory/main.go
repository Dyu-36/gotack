package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/mcp"
	"github.com/Dyu-36/gotack/internal/memory"
)

const (
	serverName    = "gotack-memory"
	serverVersion = "0.2.0"
)

func main() {
	dir := flag.String("dir", "", "personal context directory (default: <appconfig dir>/assistant)")
	flag.Parse()
	if *dir == "" {
		if err := memory.EnsureAssistant(appconfig.Dir()); err != nil {
			fmt.Fprintf(os.Stderr, "memory: legacy import deferred; originals preserved: %v\n", err)
			os.Exit(1)
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	server := &mcp.Server{
		Name: serverName, Version: serverVersion,
		Tools: []mcp.Tool{memory.Tool(memory.NewStore(resolveDir(*dir)))},
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
	return memory.Directory(appconfig.Dir())
}
