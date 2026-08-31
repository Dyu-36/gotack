package schedule

// job.go -- role: the schedule.json model: job definitions, spec validation,
// due-time computation and atomic persistence.
//
// The desktop host schedules agent runs but never executes agent logic
// itself (ADR 0001): a firing is one session plus one prompt submitted over
// the same REST path the UI uses, and its outcome arrives over SSE.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// FileName is the schedule file's name inside the Gotack config directory,
// matching the existing zalo.json convention.
const FileName = "schedule.json"

// minInterval bounds "every" specs: firings closer than one minute apart
// would only fight the hourly budget and spam the engine.
const minInterval = time.Minute

// Job is one persistent scheduled-run definition plus the host's
// bookkeeping about it. Every field is honoured by the runner; there is
// nothing the host accepts and then ignores (hard rule 8).
type Job struct {
	ID     string `json:"id"`
	Name   string `json:"name"`   // session title used for launched runs
	Prompt string `json:"prompt"` // text submitted to the engine on fire
	// Exactly one of Every and At is set.
	Every        string `json:"every,omitempty"` // Go duration ("30m"), fires repeatedly
	At           string `json:"at,omitempty"`    // local 24h "HH:MM", fires once a day
	Enabled      bool   `json:"enabled"`
	HourlyBudget int    `json:"hourly_budget,omitempty"` // 0 means DefaultHourlyBudget

	// Bookkeeping, written by the host and persisted across restarts.
	LastRun             *time.Time  `json:"last_run,omitempty"`
	LastOutcome         string      `json:"last_outcome,omitempty"`
	ConsecutiveFailures int         `json:"consecutive_failures,omitempty"`
	DisabledReason      string      `json:"disabled_reason,omitempty"`
	RecentFires         []time.Time `json:"recent_fires,omitempty"`
}

// File is the on-disk shape of schedule.json.
type File struct {
	Jobs []*Job `json:"jobs"`
}

// ValidateJob checks one job definition. The error text names the offending
// field so a hand-edited schedule.json is easy to repair.
func ValidateJob(job *Job) error {
	if job.ID == "" {
		return errors.New("schedule: job id is required")
	}
	if job.Prompt == "" {
		return fmt.Errorf("schedule: job %s: prompt is required", job.ID)
	}
	hasEvery, hasAt := job.Every != "", job.At != ""
	if hasEvery == hasAt {
		return fmt.Errorf("schedule: job %s: set exactly one of every/at", job.ID)
	}
	if hasEvery {
		d, err := time.ParseDuration(job.Every)
		if err != nil || d < minInterval {
			return fmt.Errorf("schedule: job %s: every must be a duration of at least %s", job.ID, minInterval)
		}
	} else if _, err := time.Parse("15:04", job.At); err != nil {
		return fmt.Errorf("schedule: job %s: at must be a 24h HH:MM time", job.ID)
	}
	if job.HourlyBudget < 0 {
		return fmt.Errorf("schedule: job %s: hourly_budget must be >= 0", job.ID)
	}
	return nil
}

// ValidateFile checks the whole file, including cross-job constraints.
func ValidateFile(file *File) error {
	seen := make(map[string]struct{}, len(file.Jobs))
	for _, job := range file.Jobs {
		if err := ValidateJob(job); err != nil {
			return err
		}
		if _, dup := seen[job.ID]; dup {
			return fmt.Errorf("schedule: duplicate job id %q", job.ID)
		}
		seen[job.ID] = struct{}{}
	}
	return nil
}

// NextDue reports when job is next due relative to now. Interval jobs are
// due at last run plus the interval, or immediately when never run.
// Time-of-day jobs are due at today's occurrence until that occurrence has
// been run — a missed occurrence (host down at the time) catches up once —
// and at tomorrow's occurrence afterwards.
func NextDue(job *Job, now time.Time) time.Time {
	if job.Every != "" {
		d, err := time.ParseDuration(job.Every)
		if err != nil {
			// Validation owns malformed specs; never postpone because of one.
			return now
		}
		if job.LastRun == nil {
			return now
		}
		return job.LastRun.Add(d)
	}
	occ := occurrenceToday(job.At, now)
	if job.LastRun != nil && !job.LastRun.Before(occ) {
		return occ.AddDate(0, 0, 1)
	}
	return occ
}

// occurrenceToday builds today's HH:MM in the local clock the host runs on.
func occurrenceToday(at string, now time.Time) time.Time {
	parsed, err := time.Parse("15:04", at)
	if err != nil {
		return now
	}
	return time.Date(now.Year(), now.Month(), now.Day(), parsed.Hour(), parsed.Minute(), 0, 0, now.Location())
}

// LoadFile reads schedule.json. A missing file is an empty schedule, not an
// error; a malformed file is an error, because silently discarding it would
// drop user jobs and their bookkeeping.
func LoadFile(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &File{}, nil
		}
		return nil, fmt.Errorf("schedule: read %s: %w", path, err)
	}
	if len(data) == 0 {
		return &File{}, nil
	}
	var file File
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("schedule: parse %s: %w", path, err)
	}
	return &file, nil
}

// SaveFile persists schedule.json atomically (temp file plus rename), the
// same precedent as zalo.json, so a crash mid-write can never leave a
// truncated schedule behind. Fire records older than the budget window are
// pruned because nothing will ever count them again.
func SaveFile(path string, file *File, now time.Time) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("schedule: mkdir: %w", err)
	}
	for _, job := range file.Jobs {
		job.RecentFires = pruneFires(job.RecentFires, now)
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("schedule: encode: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("schedule: write temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("schedule: replace: %w", err)
	}
	return nil
}
