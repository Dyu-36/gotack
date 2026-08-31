// guard is the Gotack PreToolUse approval hook. Crush spawns it before every
// tool call, piping the hook payload to stdin; guard writes its decision to
// stdout. The decision logic lives in internal/guard so it is unit-testable;
// this binary only performs the stdin/stdout round-trip. It never prompts and
// never blocks: a hook can only allow, deny, or stay silent.
package main

import (
	"io"
	"os"

	"github.com/Dyu-36/gotack/internal/guard"
)

func main() {
	if err := run(); err != nil {
		// A failing hook must not block the agent: Crush treats a non-zero
		// exit (other than 2/49) as a non-blocking error and proceeds. Report
		// on stderr and exit 1 so the floor failing is visible, not silent.
		os.Stderr.WriteString("gotack-guard: " + err.Error() + "\n")
		os.Exit(1)
	}
}

func run() error {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	out := guard.Evaluate(guard.ParseInput(data))
	payload, err := guard.MarshalOutput(out)
	if err != nil {
		return err
	}
	if len(payload) == 0 {
		return nil // pass-through: no opinion
	}
	_, err = os.Stdout.Write(payload)
	return err
}
