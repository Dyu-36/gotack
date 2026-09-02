package schedule

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const FileName = "schedule.json"

const minInterval = time.Minute

type Job struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Prompt string `json:"prompt"`

	Every        string `json:"every,omitempty"`
	At           string `json:"at,omitempty"`
	Enabled      bool   `json:"enabled"`
	HourlyBudget int    `json:"hourly_budget,omitempty"`

	LastRun             *time.Time  `json:"last_run,omitempty"`
	LastOutcome         string      `json:"last_outcome,omitempty"`
	ConsecutiveFailures int         `json:"consecutive_failures,omitempty"`
	DisabledReason      string      `json:"disabled_reason,omitempty"`
	RecentFires         []time.Time `json:"recent_fires,omitempty"`
}

type File struct {
	Jobs []*Job `json:"jobs"`
}

func ValidateJob(job *Job) error {
	if job == nil {
		return errors.New("schedule: job must not be null")
	}
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

func ValidateFile(file *File) error {
	if file == nil {
		return errors.New("schedule: file must not be null")
	}
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

func NextDue(job *Job, now time.Time) time.Time {
	if job.Every != "" {
		d, err := time.ParseDuration(job.Every)
		if err != nil {

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

func occurrenceToday(at string, now time.Time) time.Time {
	parsed, err := time.Parse("15:04", at)
	if err != nil {
		return now
	}
	return time.Date(now.Year(), now.Month(), now.Day(), parsed.Hour(), parsed.Minute(), 0, 0, now.Location())
}

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

func SaveFile(path string, file *File, now time.Time) error {
	if file == nil {
		return errors.New("schedule: file must not be null")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("schedule: mkdir: %w", err)
	}
	for _, job := range file.Jobs {
		if job == nil {
			return errors.New("schedule: job must not be null")
		}
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
