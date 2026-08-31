package schedule

import (
	"testing"
	"time"
)

// budget_test.go -- role: table-driven proofs for the per-job hourly budget,
// the cap that keeps scheduled firings from ever spamming the engine.

func TestBudgetFor(t *testing.T) {
	cases := []struct {
		name string
		job  Job
		want int
	}{
		{"unset uses default", Job{}, DefaultHourlyBudget},
		{"explicit budget", Job{HourlyBudget: 5}, 5},
		{"one stays one", Job{HourlyBudget: 1}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := budgetFor(&tc.job); got != tc.want {
				t.Fatalf("budgetFor = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestFiresWithinWindow(t *testing.T) {
	now := base2026()
	cases := []struct {
		name  string
		fires []time.Time
		since time.Duration
		want  int
	}{
		{"empty", nil, time.Hour, 0},
		{"all inside", []time.Time{now.Add(-5 * time.Minute), now.Add(-30 * time.Minute)}, time.Hour, 2},
		{"boundary excluded", []time.Time{now.Add(-time.Hour)}, time.Hour, 0},
		{"older excluded", []time.Time{now.Add(-2 * time.Hour), now.Add(-10 * time.Minute)}, time.Hour, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := firesWithin(tc.fires, now.Add(-tc.since)); got != tc.want {
				t.Fatalf("firesWithin = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestBudgetAllowsAndCaps(t *testing.T) {
	now := base2026()
	cases := []struct {
		name   string
		budget int
		fires  []time.Time
		want   bool
	}{
		{"no fires yet", 2, nil, true},
		{"under cap", 2, []time.Time{now.Add(-20 * time.Minute)}, true},
		{"at cap denies", 2, []time.Time{now.Add(-20 * time.Minute), now.Add(-5 * time.Minute)}, false},
		{"window slides", 2, []time.Time{now.Add(-90 * time.Minute), now.Add(-80 * time.Minute)}, true},
		{"default cap applies", 0, []time.Time{now.Add(-5 * time.Minute), now.Add(-4 * time.Minute)}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			job := &Job{ID: "j", Prompt: "p", Every: "5m", HourlyBudget: tc.budget, RecentFires: tc.fires}
			if got := budgetAllows(job, now); got != tc.want {
				t.Fatalf("budgetAllows = %v, want %v", got, tc.want)
			}
		})
	}
}
