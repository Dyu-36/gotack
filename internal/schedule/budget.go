package schedule

// budget.go -- role: the per-job hourly budget. The budget caps how many
// runs a single job may launch inside a sliding one-hour window, so a
// misconfigured job (or a firing loop after clock or bookkeeping drift) can
// never spam the engine. Budget counts launched runs; a launch that fails
// before a run starts is governed by the retry policy instead, which is
// itself bounded by the failure threshold.

import "time"

// DefaultHourlyBudget applies when a job sets no explicit hourly_budget.
// Two launches per hour is enough for retry headroom under the default
// failure threshold while keeping any runaway job expensive to nobody.
const DefaultHourlyBudget = 2

// budgetWindow is the sliding window the budget counts inside.
const budgetWindow = time.Hour

// budgetFor resolves the effective budget of a job.
func budgetFor(job *Job) int {
	if job.HourlyBudget <= 0 {
		return DefaultHourlyBudget
	}
	return job.HourlyBudget
}

// firesWithin counts fires strictly after the given bound.
func firesWithin(fires []time.Time, since time.Time) int {
	n := 0
	for _, f := range fires {
		if f.After(since) {
			n++
		}
	}
	return n
}

// budgetAllows reports whether job may launch one more run now without
// exceeding its hourly budget.
func budgetAllows(job *Job, now time.Time) bool {
	return firesWithin(job.RecentFires, now.Add(-budgetWindow)) < budgetFor(job)
}

// pruneFires drops fire records outside the budget window. Reuses the
// backing array: callers must treat the input as consumed.
func pruneFires(fires []time.Time, now time.Time) []time.Time {
	since := now.Add(-budgetWindow)
	out := fires[:0]
	for _, f := range fires {
		if f.After(since) {
			out = append(out, f)
		}
	}
	return out
}
