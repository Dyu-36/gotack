package schedule

// scheduler.go -- role: the scheduled-run runner.
//
// Semantics:
//   - A firing submits ONE agent run through the desktop Runtime seam (create
//     session, mark unattended, send prompt). The host never runs agent logic
//     itself (ADR 0001).
//   - Every scheduled session is marked unattended in the guard roster BEFORE
//     the prompt is submitted, so the WP6 posture applies: ask-tier calls are
//     denied with a legible reason instead of hanging on a prompt nobody can
//     answer. A failed mark aborts the firing.
//   - Outcomes arrive through RecordOutcome, fed by the host from the
//     run_complete SSE event. There is no polling anywhere in this package:
//     engine readiness is pushed via SetEngineReady, completions via SSE.
//   - A job never fires concurrently with itself, never re-fires inside its
//     interval window, and last-run bookkeeping is persisted before the run
//     starts so the guard survives restarts.
//   - Failed launches are recorded, retried after a backoff, and disable the
//     job after a consecutive-failure threshold; they are never silently
//     dropped and never spam, because the budget and the backoff both bound
//     them.

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
	// defaultFailureThreshold disables a job after this many consecutive
	// failures (launch failures and run errors both count), the failure-nudge
	// borrowed from Hermes (plan 5.1).
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
	SendPrompt     func(ctx context.Context, sessionID, prompt string) (runID string, err error)
	// Preflight skips a firing instead of burning a failed run (for example
	// when no provider/model is configured); its error never counts as a
	// failure strike.
	Preflight func(ctx context.Context) error
}

// flight tracks one launched run until its run_complete arrives.
type flight struct {
	sessionID string
	runID     string
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
	s.done = make(chan struct{})
	s.mu.Unlock()
	go s.loop(loopCtx)
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
	if !ready {
		return
	}
	if started {
		select {
		case s.wake <- struct{}{}:
		default:
		}
		return
	}
	// With no loop running yet there is nobody to consume the wake signal,
	// so evaluate directly to keep readiness from being stranded.
	s.evaluate(context.Background())
}

// RecordOutcome consumes one run completion. The host feeds it from the
// run_complete SSE payload (never by polling). A session that is not an
// in-flight scheduled run is ignored: it belongs to the UI or Zalo.
func (s *Scheduler) RecordOutcome(sessionID, runErr string, cancelled bool) {
	if sessionID == "" {
		return
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
			return
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
		s.persistLocked(job.ID)
		return
	}
}

func (s *Scheduler) loop(ctx context.Context) {
	defer close(s.done)
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
	if !s.engineReady {
		s.mu.Unlock()
		return
	}
	var due []*Job
	for _, job := range s.file.Jobs {
		if s.dueLocked(job, now) {
			due = append(due, job)
		}
	}
	s.mu.Unlock()
	for _, job := range due {
		s.fire(ctx, job)
	}
}

// dueLocked reports whether job may launch now. Callers hold s.mu.
func (s *Scheduler) dueLocked(job *Job, now time.Time) bool {
	if !job.Enabled || strings.TrimSpace(job.Prompt) == "" {
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

// fire launches one agent run for job. The call order is pinned by tests:
// create session, mark it unattended, then submit the prompt. A failed mark
// aborts the firing: an unmarked scheduled session could hang on an
// approval prompt nobody can answer (ADR 0002, plan 5.4).
func (s *Scheduler) fire(ctx context.Context, job *Job) {
	fctx, cancel := context.WithTimeout(ctx, s.fireTimeout)
	defer cancel()

	if s.rt.Preflight != nil {
		if err := s.rt.Preflight(fctx); err != nil {
			s.noteSkip(job, err)
			return
		}
	}

	now := s.now()
	s.mu.Lock()
	if _, busy := s.inflight[job.ID]; busy {
		s.mu.Unlock()
		return
	}
	prevRun := job.LastRun
	// Claim the firing before any network call so the interval guard holds
	// even if the host dies mid-launch; the claim is rolled back on failure
	// so the retry policy can try again.
	job.LastRun = &now
	job.LastOutcome = "fired"
	s.inflight[job.ID] = &flight{}
	s.persistLocked(job.ID)
	s.mu.Unlock()

	title := "Schedule: " + strings.TrimSpace(job.Name)
	if strings.TrimSpace(job.Name) == "" {
		title = "Schedule: " + job.ID
	}
	sessionID, err := s.rt.CreateSession(fctx, title)
	if err == nil {
		err = s.rt.MarkUnattended(fctx, sessionID)
	}
	var runID string
	if err == nil {
		runID, err = s.rt.SendPrompt(fctx, sessionID, job.Prompt)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.inflight, job.ID)
	if err != nil {
		s.recordLaunchFailureLocked(job, prevRun, err)
		return
	}
	delete(s.retryAfter, job.ID)
	job.RecentFires = append(pruneFires(job.RecentFires, now), now)
	s.inflight[job.ID] = &flight{sessionID: sessionID, runID: runID}
	s.persistLocked(job.ID)
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
	s.persistLocked(job.ID)
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
func (s *Scheduler) noteSkip(job *Job, err error) {
	reason := "skipped: " + err.Error()
	s.mu.Lock()
	defer s.mu.Unlock()
	if job.LastOutcome == reason {
		return
	}
	job.LastOutcome = reason
	s.persistLocked(job.ID)
}

func (s *Scheduler) jobLocked(id string) *Job {
	for _, job := range s.file.Jobs {
		if job.ID == id {
			return job
		}
	}
	return nil
}

// persistLocked saves the job file; a save failure is logged, never fatal,
// because the in-memory state stays authoritative for this host run.
func (s *Scheduler) persistLocked(jobID string) {
	if err := SaveFile(s.path, &s.file, s.now()); err != nil {
		s.log.Warn("schedule: persist failed", "job", jobID, "err", err)
	}
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
