package guard

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const UnattendedRosterFileName = "unattended-sessions.json"

const ReviewRosterFileName = "review-sessions.json"

const rosterCap = 500

type rosterFile struct {
	Sessions []string `json:"sessions"`
}

var rosterMu sync.Mutex

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

func ReviewRosterContains(path, sessionID string) bool {
	return RosterContains(path, sessionID)
}

func MarkUnattendedSession(path, sessionID string) error {
	rosterMu.Lock()
	defer rosterMu.Unlock()
	return markSession(path, sessionID)
}

func MarkReviewSession(path, sessionID string) error {
	rosterMu.Lock()
	defer rosterMu.Unlock()
	return markSession(path, sessionID)
}

func UnmarkReviewSession(path, sessionID string) error {
	if sessionID == "" {
		return errors.New("session id is required")
	}
	rosterMu.Lock()
	defer rosterMu.Unlock()
	sessions := loadRoster(path)
	for i, id := range sessions {
		if id != sessionID {
			continue
		}
		sessions = append(sessions[:i], sessions[i+1:]...)
		return writeRoster(path, sessions)
	}
	return nil
}

func markSession(path, sessionID string) error {
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
	return writeRoster(path, sessions)
}

func writeRoster(path string, sessions []string) error {
	data, err := json.MarshalIndent(rosterFile{Sessions: sessions}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode roster: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir roster dir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create roster temp file: %w", err)
	}
	tmpPath := tmp.Name()
	if err = tmp.Chmod(0o644); err == nil {
		_, err = tmp.Write(data)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmpPath, path)
	}
	if err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write roster: %w", err)
	}
	return nil
}

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
