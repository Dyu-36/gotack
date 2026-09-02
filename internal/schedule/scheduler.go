package schedule

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const (
	// defaultTick is how often the loop re-evaluates due jobs. Time-based
	// firing has no event source, so an internal clock is required; engine
	// state is never polled here (readiness is pushed, outcomes ride SSE).
	defaultTick = 30 * time.Second
	// Launch failures and run errors both count toward this threshold.
	defaultFailureThreshold = 3
	// defaultRetryDelay is the backoff before a failed launch may retry.
	defaultRetryDelay = 5 * time.Minute
	// defaultFireTimeout bounds one launch sequence (create, mark, send).
	defaultFireTimeout = 30 * time.Second
)

// Runtime is the desktop seam the scheduler fires through. The host supplies
// implementations over internal/crushapi and the guard roster; Crush stays
// the only executor of agent logic.
type Runtime struct {
	CreateSession  func(ctx context.Context, title string) (sessionID string, err error)
	MarkUnattended func(ctx context.Context, sessionID string) error
	SendPrompt     func(ctx context.Context, sessionID, prompt string) error
	// Preflight skips a firing instead of burning a failed run (for example
	// when no provider/model is configured); its error never counts as a
	// failure strike.
	Preflight func(ctx context.Context) error
}

// flight tracks one launched run until its run_complete arrives.
type flight struct {
	sessionID string
}

// jobSnapshot is the immutable launch input captured while s.mu is held.
// Runtime calls may block for seconds; passing a *Job beyond that lock would
// race with outcome bookkeeping (and, on restart, a fresh file load).
type jobSnapshot struct {
	id     string
	name   string
	prompt string
}

// Scheduler owns the job file, the tick loop and the firing guards.
type Scheduler struct {
	path string
	rt   Runtime
	log  *slog.Logger

	now           func() time.Time
	tick          time.Duration
	failThreshold int
	retryDelay    time.Duration
	fireTimeout   time.Duration

	mu          sync.Mutex
	file        File
	engineReady bool
	started     bool
	inflight    map[string]*flight
	retryAfter  map[string]time.Time
	wake        chan struct{}
	cancel      context.CancelFunc
	done        chan struct{}
}

// New builds a Scheduler persisting to path. Constructing performs no I/O.
func New(path string, rt Runtime, log *slog.Logger) *Scheduler {
	if log == nil {
		log = slog.Default()
	}
	return &Scheduler{
		path:          path,
		rt:            rt,
		log:           log,
		now:           time.Now,
		tick:          defaultTick,
		failThreshold: defaultFailureThreshold,
		retryDelay:    defaultRetryDelay,
		fireTimeout:   defaultFireTimeout,
		inflight:      make(map[string]*flight),
		retryAfter:    make(map[string]time.Time),
		wake:          make(chan struct{}, 1),
	}
}

// Start loads schedule.json and runs the evaluation loop until Stop or the
// parent context ends. An unparsable file never crashes the host: the
// scheduler runs with no jobs, the file stays on disk, and scheduling
// resumes on the next start once it parses again.
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return errors.New("schedule: scheduler already started")
	}
	if err := s.loadLocked(); err != nil {
		s.log.Error("schedule: starting without jobs, schedule.json unusable", "err", err)
		s.file = File{}
	}
	s.started = true
	loopCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	done := make(chan struct{})
	s.done = done
	ready := s.engineReady
	s.mu.Unlock()
	go s.loop(loopCtx, done)
	if ready {
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
	return nil
}

// Stop ends the loop and waits for it to exit.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	done := s.done
	s.cancel = nil
	s.done = nil
	s.started = false
	s.engineReady = false
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

// SetEngineReady is the host's push signal for engine availability: true
// when the connection flow commits, false when the transport stops or is
// lost. A transition to ready re-evaluates due jobs immediately, so an
// overdue firing does not wait for the next tick after a reconnect.
func (s *Scheduler) SetEngineReady(ready bool) {
	s.mu.Lock()
	s.engineReady = ready
	started := s.started
	s.mu.Unlock()
	if !ready || !started {
		return
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// RecordOutcome consumes one run completion. The host feeds it from the
// run_complete SSE payload (never by polling). It reports whether the session
// belonged to a scheduled run so callers can keep Hermes' background reviewer
// disabled for cron-style work; UI and Zalo sessions return false.
func (s *Scheduler) RecordOutcome(sessionID, runErr string, cancelled bool) bool {
	if sessionID == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, fl := range s.inflight {
		if fl.sessionID != sessionID {
			continue
		}
		delete(s.inflight, id)
		job := s.jobLocked(id)
		if job == nil {
			return true
		}
		switch {
		case cancelled:
			job.LastOutcome = "cancelled"
		case runErr != "":
			job.ConsecutiveFailures++
			job.LastOutcome = "run failed: " + runErr
			s.disableIfThresholdLocked(job)
		default:
			job.ConsecutiveFailures = 0
			job.LastOutcome = "complete"
		}
		_ = s.persistLocked(job.ID)
		return true
	}
	return false
}

func (s *Scheduler) loop(ctx context.Context, done chan struct{}) {
	defer close(done)
	ticker := time.NewTicker(s.tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.evaluate(ctx)
		case <-s.wake:
			s.evaluate(ctx)
		}
	}
}

// evaluate fires every job that is due right now. Locks are never held
// across Runtime calls, so a slow engine cannot stall bookkeeping.
func (s *Scheduler) evaluate(ctx context.Context) {
	now := s.now()
	s.mu.Lock()
	if !s.started || !s.engineReady {
		s.mu.Unlock()
		return
	}
	var due []jobSnapshot
	for _, job := range s.file.Jobs {
		if s.dueLocked(job, now) {
			due = append(due, jobSnapshot{id: job.ID, name: job.Name, prompt: job.Prompt})
		}
	}
	s.mu.Unlock()
	var launches sync.WaitGroup
	launches.Add(len(due))
	for _, job := range due {
		go func(snapshot jobSnapshot) {
			defer launches.Done()
			s.fire(ctx, snapshot)
		}(job)
	}
	launches.Wait()
}

// dueLocked reports whether job may launch now. Callers hold s.mu.
func (s *Scheduler) dueLocked(job *Job, now time.Time) bool {
	if job == nil || !job.Enabled || strings.TrimSpace(job.Prompt) == "" {
		return false
	}
	if _, busy := s.inflight[job.ID]; busy {
		return false
	}
	if backoff, ok := s.retryAfter[job.ID]; ok && now.Before(backoff) {
		return false
	}
	if !budgetAllows(job, now) {
		return false
	}
	return !now.Before(NextDue(job, now))
}

// fire launches one agent run for job. The session is marked unattended
// before its prompt is submitted so approval cannot block without a user.
func (s *Scheduler) fire(ctx context.Context, snapshot jobSnapshot) {
	fctx, cancel := context.WithTimeout(ctx, s.fireTimeout)
	defer cancel()

	if s.rt.Preflight != nil {
		if err := s.rt.Preflight(fctx); err != nil {
			s.noteSkip(snapshot.id, err)
			return
		}
	}

	now := s.now()
	s.mu.Lock()
	job := s.jobLocked(snapshot.id)
	if !s.started || !s.engineReady || job == nil || !job.Enabled || strings.TrimSpace(job.Prompt) == "" {
		s.mu.Unlock()
		return
	}
	if _, busy := s.inflight[snapshot.id]; busy {
		s.mu.Unlock()
		return
	}
	// Re-check due-ness after preflight. Another outcome or a clock advance may
	// have made this snapshot stale while the preflight call was running.
	if !s.dueLocked(job, now) {
		s.mu.Unlock()
		return
	}
	previousRun := job.LastRun
	previousOutcome := job.LastOutcome
	// Claim the firing before any network call so the interval guard holds
	// even if the host dies mid-launch; the claim is rolled back on failure
	// so the retry policy can try again.
	job.LastRun = &now
	job.LastOutcome = "fired"
	s.inflight[snapshot.id] = &flight{}
	if err := s.persistLocked(snapshot.id); err != nil {
		job.LastRun = previousRun
		job.LastOutcome = previousOutcome
		delete(s.inflight, snapshot.id)
		s.mu.Unlock()
		return
	}
	// Copy all mutable job fields needed after the lock is released.
	title := "Schedule: " + strings.TrimSpace(job.Name)
	if strings.TrimSpace(job.Name) == "" {
		title = "Schedule: " + snapshot.id
	}
	prompt := job.Prompt
	s.mu.Unlock()

	sessionID, err := s.rt.CreateSession(fctx, title)
	if err == nil {
		// Register the session before submission. A very short run may emit
		// run_complete before SendPrompt returns.
		s.mu.Lock()
		if current, ok := s.inflight[snapshot.id]; ok {
			current.sessionID = sessionID
		}
		s.mu.Unlock()
		err = s.rt.MarkUnattended(fctx, sessionID)
	}
	if err == nil {
		err = s.rt.SendPrompt(fctx, sessionID, prompt)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	job = s.jobLocked(snapshot.id)
	if err != nil {
		delete(s.inflight, snapshot.id)
		if job != nil {
			s.recordLaunchFailureLocked(job, previousRun, err)
		}
		return
	}
	delete(s.retryAfter, snapshot.id)
	if job == nil {
		delete(s.inflight, snapshot.id)
		return
	}
	job.RecentFires = append(pruneFires(job.RecentFires, now), now)
	// RecordOutcome may already have consumed the flight while SendPrompt was
	// returning. Never recreate a completed flight here.
	_ = s.persistLocked(job.ID)
}

// recordLaunchFailureLocked books a failed launch: the interval claim rolls
// back so the firing can retry after the backoff, the failure strike is
// recorded, and the job disables at the threshold. Callers hold s.mu.
func (s *Scheduler) recordLaunchFailureLocked(job *Job, prevRun *time.Time, err error) {
	job.LastRun = prevRun
	job.ConsecutiveFailures++
	job.LastOutcome = "launch failed: " + err.Error()
	s.retryAfter[job.ID] = s.now().Add(s.retryDelay)
	s.disableIfThresholdLocked(job)
	_ = s.persistLocked(job.ID)
}

func (s *Scheduler) disableIfThresholdLocked(job *Job) {
	if job.ConsecutiveFailures < s.failThreshold {
		return
	}
	job.Enabled = false
	job.DisabledReason = fmt.Sprintf("disabled after %d consecutive failures", job.ConsecutiveFailures)
	delete(s.retryAfter, job.ID)
}

// noteSkip records a preflight skip without burning a failure strike; it is
// visible in the outcome so the skip is never silent. Callers hold no lock.
func (s *Scheduler) noteSkip(jobID string, err error) {
	reason := "skipped: " + err.Error()
	s.mu.Lock()
	defer s.mu.Unlock()
	job := s.jobLocked(jobID)
	if job == nil {
		return
	}
	if job.LastOutcome == reason {
		return
	}
	job.LastOutcome = reason
	_ = s.persistLocked(job.ID)
}

func (s *Scheduler) jobLocked(id string) *Job {
	for _, job := range s.file.Jobs {
		if job == nil {
			continue
		}
		if job.ID == id {
			return job
		}
	}
	return nil
}

// persistLocked saves the job file and lets claim-time callers abort before
// network I/O when the restart guard could not be recorded.
func (s *Scheduler) persistLocked(jobID string) error {
	if err := SaveFile(s.path, &s.file, s.now()); err != nil {
		s.log.Warn("schedule: persist failed", "job", jobID, "err", err)
		return err
	}
	return nil
}

// load reads and validates the job file. Callers hold s.mu.
func (s *Scheduler) loadLocked() error {
	file, err := LoadFile(s.path)
	if err != nil {
		return err
	}
	if err := ValidateFile(file); err != nil {
		return err
	}
	now := s.now()
	for _, job := range file.Jobs {
		// A job hand-edited back to enabled starts a clean failure streak.
		if job.Enabled {
			job.DisabledReason = ""
			job.ConsecutiveFailures = 0
		}
		job.RecentFires = pruneFires(job.RecentFires, now)
	}
	s.file = *file
	return nil
}

// load is the lock-taking wrapper used by tests and restart scenarios.
func (s *Scheduler) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

// inflightCount reports how many scheduled runs await their run_complete.
func (s *Scheduler) inflightCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.inflight)
}
