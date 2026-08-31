package schedule

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// job_test.go -- role: proofs for the schedule.json model: spec validation,
// due-time computation, persistence round-trip and atomic writes.

func mustTime(t *testing.T, value string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse %q: %v", value, err)
	}
	return ts
}

func TestJobValidation(t *testing.T) {
	valid := Job{ID: "j1", Name: "daily", Prompt: "do work", Every: "30m", Enabled: true}
	cases := []struct {
		name    string
		mutate  func(*Job)
		wantErr string
	}{
		{"valid interval", func(j *Job) {}, ""},
		{"valid time of day", func(j *Job) { j.Every = ""; j.At = "08:30" }, ""},
		{"missing id", func(j *Job) { j.ID = "" }, "id"},
		{"missing prompt", func(j *Job) { j.Prompt = "" }, "prompt"},
		{"no spec", func(j *Job) { j.Every = "" }, "one of"},
		{"both specs", func(j *Job) { j.At = "08:30" }, "one of"},
		{"bad duration", func(j *Job) { j.Every = "soon" }, "every"},
		{"interval too short", func(j *Job) { j.Every = "30s" }, "every"},
		{"bad time of day", func(j *Job) { j.Every = ""; j.At = "8am" }, "at"},
		{"time of day out of range", func(j *Job) { j.Every = ""; j.At = "25:00" }, "at"},
		{"negative budget", func(j *Job) { j.HourlyBudget = -1 }, "hourly_budget"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			job := valid
			tc.mutate(&job)
			err := ValidateJob(&job)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected valid job, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not mention %q", err, tc.wantErr)
			}
		})
	}
}

func TestValidateFileRejectsDuplicateIDs(t *testing.T) {
	file := &File{Jobs: []*Job{
		{ID: "dup", Prompt: "a", Every: "10m"},
		{ID: "dup", Prompt: "b", Every: "20m"},
	}}
	err := ValidateFile(file)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate id error, got %v", err)
	}
}

func TestNextDue(t *testing.T) {
	base := mustTime(t, "2026-09-01T12:00:00+07:00")
	at830 := time.Date(base.Year(), base.Month(), base.Day(), 8, 30, 0, 0, base.Location())

	cases := []struct {
		name string
		job  Job
		last *time.Time
		want time.Time
	}{
		{
			name: "interval never run fires immediately",
			job:  Job{Every: "30m"},
			want: base,
		},
		{
			name: "interval not yet elapsed",
			job:  Job{Every: "30m"},
			last: ptrTime(base.Add(-10 * time.Minute)),
			want: base.Add(20 * time.Minute),
		},
		{
			name: "interval elapsed",
			job:  Job{Every: "30m"},
			last: ptrTime(base.Add(-31 * time.Minute)),
			want: base.Add(-1 * time.Minute),
		},
		{
			name: "time of day still ahead today",
			job:  Job{At: "18:00"},
			want: time.Date(base.Year(), base.Month(), base.Day(), 18, 0, 0, 0, base.Location()),
		},
		{
			name: "time of day missed earlier today catches up",
			job:  Job{At: "08:30"},
			want: at830,
		},
		{
			name: "time of day already ran today waits for tomorrow",
			job:  Job{At: "08:30"},
			last: ptrTime(at830.Add(5 * time.Minute)),
			want: at830.AddDate(0, 0, 1),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.job.LastRun = tc.last
			got := NextDue(&tc.job, base)
			if !got.Equal(tc.want) {
				t.Fatalf("NextDue = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	// Fire records inside the budget window survive; LastRun is plain
	// bookkeeping and has no window.
	last := base2026().Add(-10 * time.Minute)
	fire := base2026().Add(-9*time.Minute - 58*time.Second)
	in := &File{Jobs: []*Job{
		{
			ID: "daily", Name: "Daily digest", Prompt: "summarise", At: "08:30",
			Enabled: true, HourlyBudget: 1,
			LastRun: &last, LastOutcome: "complete",
			RecentFires: []time.Time{fire},
		},
		{
			ID: "watcher", Name: "Watcher", Prompt: "check", Every: "45m",
			Enabled: false, DisabledReason: "disabled after 3 consecutive failures",
			ConsecutiveFailures: 3,
		},
	}}
	if err := SaveFile(path, in, base2026()); err != nil {
		t.Fatalf("save: %v", err)
	}
	out, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(out.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(out.Jobs))
	}
	got := out.Jobs[0]
	if got.ID != "daily" || got.Name != "Daily digest" || got.Prompt != "summarise" || got.At != "08:30" {
		t.Fatalf("identity fields lost: %+v", got)
	}
	if !got.Enabled || got.HourlyBudget != 1 {
		t.Fatalf("enabled/budget lost: %+v", got)
	}
	if got.LastRun == nil || !got.LastRun.Equal(last) {
		t.Fatalf("last run lost: %+v", got.LastRun)
	}
	if got.LastOutcome != "complete" || len(got.RecentFires) != 1 || !got.RecentFires[0].Equal(fire) {
		t.Fatalf("bookkeeping lost: %+v", got)
	}
	disabled := out.Jobs[1]
	if disabled.Enabled || disabled.DisabledReason == "" || disabled.ConsecutiveFailures != 3 {
		t.Fatalf("disabled bookkeeping lost: %+v", disabled)
	}
}

func TestLoadFileMissingIsEmpty(t *testing.T) {
	file, err := LoadFile(filepath.Join(t.TempDir(), FileName))
	if err != nil {
		t.Fatalf("missing file must not error: %v", err)
	}
	if len(file.Jobs) != 0 {
		t.Fatalf("missing file must load empty, got %d jobs", len(file.Jobs))
	}
}

func TestLoadFileMalformedErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFile(path); err == nil {
		t.Fatal("malformed schedule.json must return an error, not load silently")
	}
}

func TestSaveFileIsAtomicAndClean(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	file := &File{Jobs: []*Job{{ID: "j", Prompt: "p", Every: "10m"}}}
	if err := SaveFile(path, file, base2026()); err != nil {
		t.Fatalf("save: %v", err)
	}
	// The temp file must not survive the rename.
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file left behind: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var parsed File
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("saved file is not valid JSON: %v", err)
	}
	// Overwriting an existing file must keep producing valid JSON.
	file.Jobs[0].LastOutcome = "complete"
	if err := SaveFile(path, file, base2026()); err != nil {
		t.Fatalf("second save: %v", err)
	}
	if data2, err := os.ReadFile(path); err != nil {
		t.Fatal(err)
	} else if err := json.Unmarshal(data2, &parsed); err != nil {
		t.Fatalf("overwritten file is not valid JSON: %v", err)
	}
}

func TestSaveFilePrunesStaleFires(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	now := base2026()
	job := &Job{
		ID: "j", Prompt: "p", Every: "10m",
		RecentFires: []time.Time{now.Add(-2 * time.Hour), now.Add(-10 * time.Minute)},
	}
	if err := SaveFile(path, &File{Jobs: []*Job{job}}, now); err != nil {
		t.Fatalf("save: %v", err)
	}
	out, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Jobs[0].RecentFires) != 1 {
		t.Fatalf("stale fire not pruned: %v", out.Jobs[0].RecentFires)
	}
	if !out.Jobs[0].RecentFires[0].Equal(now.Add(-10 * time.Minute)) {
		t.Fatalf("wrong fire pruned: %v", out.Jobs[0].RecentFires)
	}
}

func ptrTime(ts time.Time) *time.Time { return &ts }

func base2026() time.Time {
	return time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
}
