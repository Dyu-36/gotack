package schedule

import "time"

const DefaultHourlyBudget = 2

const budgetWindow = time.Hour

func budgetFor(job *Job) int {
	if job.HourlyBudget <= 0 {
		return DefaultHourlyBudget
	}
	return job.HourlyBudget
}

func firesWithin(fires []time.Time, since time.Time) int {
	n := 0
	for _, f := range fires {
		if f.After(since) {
			n++
		}
	}
	return n
}

func budgetAllows(job *Job, now time.Time) bool {
	return firesWithin(job.RecentFires, now.Add(-budgetWindow)) < budgetFor(job)
}

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
