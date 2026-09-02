// guard is the Gotack PreToolUse approval hook. Crush spawns it before every
// tool call, piping the hook payload to stdin; guard writes its decision to
// stdout. The decision logic lives in internal/guard so it is unit-testable;
// this binary only resolves the per-session options and performs the
// stdin/stdout round-trip. It never prompts and never blocks: a hook can only
// allow, deny, or stay silent.
package main

import (
	"io"
	"os"
	"path/filepath"

	"github.com/Dyu-36/gotack/internal/appconfig"
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
	in := guard.ParseInput(data)
	out := guard.Evaluate(in, optionsFor(in))
	if out.Decision == guard.DecisionDeny {
		// Every deny is logged on stderr: the refusal reason must be visible
		// beyond the tool result, above all for unattended sessions where no
		// human watches the UI.
		os.Stderr.WriteString(out.Reason + "\n")
	}
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

// optionsFor derives the per-session policy options. The write-safe root is
// the session's own working directory (the workspace path), the memory
// context dir is the fixed appconfig location, and the two rosters record
// which sessions are unattended and which are detached background reviews.
func optionsFor(in guard.Input) guard.Options {
	dir := appconfig.Dir()
	return guard.Options{
		WriteSafeRoot: in.CWD,
		ContextDir:    filepath.Join(dir, "context"),
		Unattended:    guard.RosterContains(filepath.Join(dir, guard.UnattendedRosterFileName), in.SessionID),
		Review:        guard.ReviewRosterContains(filepath.Join(dir, guard.ReviewRosterFileName), in.SessionID),
	}
}
