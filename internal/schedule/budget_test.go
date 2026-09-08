package schedule

import (
	"testing"
	"time"
)

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
		{"boundary excluded", 1, []time.Time{now.Add(-time.Hour)}, true},
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
