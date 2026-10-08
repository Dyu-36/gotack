package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/Dyu-36/gotack/internal/agentcore"
	"github.com/Dyu-36/gotack/internal/engine"
	"github.com/Dyu-36/gotack/internal/terminal"
)

func main() {
	var err error
	args := os.Args[1:]
	if len(args) > 0 && (args[0] == "serve" || args[0] == "server") {
		err = engine.ConfigureServerEnvironment()
		if err == nil {
			err = agentcore.Serve(args)
		}
	} else {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		err = terminal.Run(ctx, args, os.Stdin, os.Stdout, os.Stderr)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		if errors.Is(err, context.Canceled) {
			os.Exit(130)
		}
		os.Exit(1)
	}
}
