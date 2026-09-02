package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/mcp"
	"github.com/Dyu-36/gotack/internal/skillmanage"
)

const (
	rootEnv       = "GOTACK_SKILLS_DIR"
	serverName    = "gotack-skills"
	serverVersion = "1.0.0"
)

func main() {
	root := flag.String("root", "", "managed skills root (default: $"+rootEnv+" or <appconfig dir>/skills)")
	flag.Parse()

	manager, err := skillmanage.New(resolveRoot(*root))
	if err != nil {
		fmt.Fprintf(os.Stderr, "skills: %v\n", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server := &mcp.Server{
		Name:    serverName,
		Version: serverVersion,
		Tools:   skillmanage.Tools(manager),
	}
	if err := server.Serve(ctx, os.Stdin, os.Stdout); err != nil && ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "skills: %v\n", err)
		os.Exit(1)
	}
}

func resolveRoot(explicit string) string {
	if value := strings.TrimSpace(explicit); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv(rootEnv)); value != "" {
		return value
	}
	return filepath.Join(appconfig.Dir(), "skills")
}
