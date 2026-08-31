package guard

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// roster.go -- role: the unattended-session roster.
//
// The host records the session id of every remotely originated (Zalo) or
// scheduled session here; the spawned guard reads the roster to learn that a
// session has no human to answer approval prompts. The file is the boundary
// between the two processes, so its name and shape are part of the contract
// (docs/contracts/gotack-approvals.md).

// UnattendedRosterFileName is the roster file's name inside the Gotack
// config directory.
const UnattendedRosterFileName = "unattended-sessions.json"

// rosterCap bounds the roster: entries beyond the cap are dropped oldest
// first, so a long-running host never grows the file without limit.
const rosterCap = 500

type rosterFile struct {
	Sessions []string `json:"sessions"`
}

// RosterContains reports whether sessionID is recorded as unattended. A
// missing, unreadable or malformed roster answers false: the guard fails
// open to the interactive posture rather than blocking on unreadable state,
// and the host re-marks the session on every remote turn anyway. An empty
// session id is never unattended.
func RosterContains(path, sessionID string) bool {
	if sessionID == "" {
		return false
	}
	for _, id := range loadRoster(path) {
		if id == sessionID {
			return true
		}
	}
	return false
}

// MarkUnattendedSession records sessionID in the roster, creating the file
// and directory when needed. Duplicate marks keep a single entry. The write
// fails loudly (never silently skipped) because a session that should be
// unattended but is not recorded could hang on a prompt nobody can answer.
func MarkUnattendedSession(path, sessionID string) error {
	if sessionID == "" {
		return errors.New("session id is required")
	}
	sessions := loadRoster(path)
	for _, id := range sessions {
		if id == sessionID {
			return nil
		}
	}
	sessions = append(sessions, sessionID)
	if len(sessions) > rosterCap {
		sessions = sessions[len(sessions)-rosterCap:]
	}
	data, err := json.MarshalIndent(rosterFile{Sessions: sessions}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode roster: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir roster dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write roster: %w", err)
	}
	return nil
}

// loadRoster reads the roster file, returning nil on any absence or parse
// failure for the fail-open reason documented on RosterContains.
func loadRoster(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return nil
	}
	var rf rosterFile
	if err := json.Unmarshal(data, &rf); err != nil {
		return nil
	}
	return rf.Sessions
}
