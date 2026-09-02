package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/mcp"
	"github.com/Dyu-36/gotack/internal/recall"
)

const (
	serverName    = "gotack-recall"
	serverVersion = "0.1.0"

	dataDirEnv = "GOTACK_CRUSH_DATA_DIR"
)

func main() {
	dataDir := flag.String("data-dir", "", "Crush data directory containing crush.db (default: $"+dataDirEnv+" or the gotack default workspace data dir)")
	indexDir := flag.String("index-dir", "", "directory for recall.db (default: <appconfig dir>/recall)")
	rebuild := flag.Bool("rebuild", false, "drop and rebuild the recall index before serving")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	resolved := resolveDirs(*dataDir, *indexDir)
	store := recall.OpenStore(resolved.dataDir, resolved.indexDir, log)
	defer func() {
		if err := store.Close(); err != nil {
			log.Warn("recall: closing index", "err", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *rebuild {
		if err := store.Rebuild(ctx); err != nil {
			log.Warn("recall: rebuild failed; searches will retry", "err", err)
		}
	} else if err := store.Sync(ctx); err != nil {
		log.Warn("recall: initial sync failed; searches will retry", "err", err)
	}

	server := &mcp.Server{
		Name:    serverName,
		Version: serverVersion,
		Tools:   []mcp.Tool{recall.Tool(store)},
	}
	if err := server.Serve(ctx, os.Stdin, os.Stdout); err != nil && ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "recall: %v\n", err)
		os.Exit(1)
	}
}

type dirs struct {
	dataDir  string
	indexDir string
}

func resolveDirs(dataDirFlag, indexDirFlag string) dirs {
	resolved := dirs{
		dataDir:  dataDirFlag,
		indexDir: indexDirFlag,
	}
	if resolved.dataDir == "" {
		resolved.dataDir = os.Getenv(dataDirEnv)
	}
	if resolved.dataDir == "" {
		resolved.dataDir = filepath.Join(appconfig.Dir(), "default-workspace-data")
	}
	if resolved.indexDir == "" {
		resolved.indexDir = filepath.Join(appconfig.Dir(), "recall")
	}
	return resolved
}
