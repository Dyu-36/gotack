package schedule

import "time"

const DefaultHourlyBudget = 2

const budgetWindow = time.Hour

func budgetAllows(job *Job, now time.Time) bool {
	budget := job.HourlyBudget
	if budget <= 0 {
		budget = DefaultHourlyBudget
	}
	since := now.Add(-budgetWindow)
	fires := 0
	for _, fire := range job.RecentFires {
		if fire.After(since) {
			fires++
			if fires >= budget {
				return false
			}
		}
	}
	return true
}

func pruneFires(fires []time.Time, now time.Time) []time.Time {
	since := now.Add(-budgetWindow)
	out := fires[:0]
	for _, fire := range fires {
		if fire.After(since) {
			out = append(out, fire)
		}
	}
	return out
}
