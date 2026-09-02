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

		os.Stderr.WriteString(out.Reason + "\n")
	}
	payload, err := guard.MarshalOutput(out)
	if err != nil {
		return err
	}
	if len(payload) == 0 {
		return nil
	}
	_, err = os.Stdout.Write(payload)
	return err
}

func optionsFor(in guard.Input) guard.Options {
	dir := appconfig.Dir()
	return guard.Options{
		WriteSafeRoot: in.CWD,
		ContextDir:    filepath.Join(dir, "context"),
		Unattended:    guard.RosterContains(filepath.Join(dir, guard.UnattendedRosterFileName), in.SessionID),
		Review:        guard.ReviewRosterContains(filepath.Join(dir, guard.ReviewRosterFileName), in.SessionID),
	}
}
