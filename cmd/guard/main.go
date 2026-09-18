package main

import (
	"io"
	"os"
	"path/filepath"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/guard"
	"github.com/Dyu-36/gotack/internal/memory"
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
	in, err := guard.ParseInput(data)
	if err != nil {
		return deny("gotack-guard: malformed hook payload - failing closed (" + err.Error() + ")")
	}
	if in.Event == "" || in.ToolName == "" || in.SessionID == "" || in.CWD == "" {
		return deny("gotack-guard: incomplete hook payload - event, tool_name, session_id and cwd are required")
	}
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

func deny(reason string) error {
	payload, err := guard.MarshalOutput(guard.Deny(reason, true))
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
		WriteSafeRoot: safeRoot(in.CWD),
		ContextDir:    memory.Directory(dir),
		Unattended:    guard.RosterContains(filepath.Join(dir, guard.UnattendedRosterFileName), in.SessionID),
		Review:        guard.ReviewRosterContains(filepath.Join(dir, guard.ReviewRosterFileName), in.SessionID),
	}
}

func safeRoot(cwd string) string {
	if cwd == "" || !filepath.IsAbs(cwd) {
		return ""
	}
	info, err := os.Stat(cwd)
	if err != nil || !info.IsDir() {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(cwd); err == nil {
		return resolved
	}
	return filepath.Clean(cwd)
}
