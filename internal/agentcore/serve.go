package agentcore

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/Dyu-36/gotack/internal/agentcore/config"
	_ "github.com/Dyu-36/gotack/internal/agentcore/dns"
	"github.com/Dyu-36/gotack/internal/agentcore/log"
	"github.com/Dyu-36/gotack/internal/agentcore/server"
)

// @title Tack Engine API
// @version 1.0
// @description Local agent runtime for Gotack. Provides workspaces, sessions, agents, tools, LSP and MCP over local IPC or development TCP.
// @contact.name Gotack contributors
// @contact.url https://github.com/Dyu-36/gotack
// @license.name FSL-1.1-MIT (inherited engine source)
// @license.url https://github.com/Dyu-36/gotack/blob/main/internal/agentcore/LICENSE.md
// @BasePath /v1
func Serve(args []string) error {
	if len(args) == 0 || args[0] != "serve" && args[0] != "server" {
		return errors.New("usage: gotack serve [--host <address>] [--data-dir <path>] [--debug]")
	}

	flags := flag.NewFlagSet("server", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	defaultHost := server.DefaultHost()
	host := flags.String("host", defaultHost, "server host (tcp, unix, or npipe)")
	flags.StringVar(host, "H", defaultHost, "server host (shorthand)")
	dataDir := flags.String("data-dir", "", "custom engine data directory")
	debug := flags.Bool("debug", false, "enable debug mode")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}

	globalDir := config.GlobalWorkspaceDir()
	cfg, err := config.Load(globalDir, *dataDir, *debug)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	// Configure rotating file logging before anything can log. Best-effort: a
	// server that cannot open its log file still serves.
	logPath := filepath.Join(globalDir, "logs", "tack-engine.log")
	if mkErr := os.MkdirAll(filepath.Dir(logPath), 0o755); mkErr == nil {
		log.Setup(logPath, *debug)
	}
	hostURL, err := server.ParseHostURL(*host)
	if err != nil {
		return fmt.Errorf("parse host: %w", err)
	}

	srv := server.NewServer(cfg, hostURL.Scheme, hostURL.Host)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	select {
	case <-sigCh:
	case err := <-errCh:
		if err != nil && !errors.Is(err, server.ErrServerClosed) {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil && !errors.Is(err, server.ErrServerClosed) {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}
