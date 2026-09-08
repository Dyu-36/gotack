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
	defaultTick = 30 * time.Second

	defaultFailureThreshold = 3

	defaultRetryDelay = 5 * time.Minute

	defaultFireTimeout = 30 * time.Second
)

type Runtime struct {
	CreateSession  func(ctx context.Context, title string) (sessionID string, err error)
	MarkUnattended func(ctx context.Context, sessionID string) error
	SendPrompt     func(ctx context.Context, sessionID, prompt string) error

	Preflight func(ctx context.Context) error
}

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
	inflight    map[string]string
	retryAfter  map[string]time.Time
	wake        chan struct{}
	cancel      context.CancelFunc
	done        chan struct{}
}

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
		inflight:      make(map[string]string),
		retryAfter:    make(map[string]time.Time),
		wake:          make(chan struct{}, 1),
	}
}

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

func (s *Scheduler) RecordOutcome(sessionID, runErr string, cancelled bool) bool {
	if sessionID == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, currentSessionID := range s.inflight {
		if currentSessionID != sessionID {
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

func (s *Scheduler) evaluate(ctx context.Context) {
	now := s.now()
	s.mu.Lock()
	if !s.started || !s.engineReady {
		s.mu.Unlock()
		return
	}
	var due []string
	for _, job := range s.file.Jobs {
		if s.dueLocked(job, now) {
			due = append(due, job.ID)
		}
	}
	s.mu.Unlock()

	var launches sync.WaitGroup
	launches.Add(len(due))
	for _, jobID := range due {
		go func() {
			defer launches.Done()
			s.fire(ctx, jobID)
		}()
	}
	launches.Wait()
}

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

func (s *Scheduler) fire(ctx context.Context, jobID string) {
	fctx, cancel := context.WithTimeout(ctx, s.fireTimeout)
	defer cancel()

	if s.rt.Preflight != nil {
		if err := s.rt.Preflight(fctx); err != nil {
			s.noteSkip(jobID, err)
			return
		}
	}

	now := s.now()
	s.mu.Lock()
	job := s.jobLocked(jobID)
	if !s.started || !s.engineReady || job == nil || !job.Enabled || strings.TrimSpace(job.Prompt) == "" {
		s.mu.Unlock()
		return
	}
	if _, busy := s.inflight[jobID]; busy {
		s.mu.Unlock()
		return
	}
	if !s.dueLocked(job, now) {
		s.mu.Unlock()
		return
	}

	previousRun := job.LastRun
	previousOutcome := job.LastOutcome
	job.LastRun = &now
	job.LastOutcome = "fired"
	s.inflight[jobID] = ""
	if err := s.persistLocked(jobID); err != nil {
		job.LastRun = previousRun
		job.LastOutcome = previousOutcome
		delete(s.inflight, jobID)
		s.mu.Unlock()
		return
	}

	name := strings.TrimSpace(job.Name)
	title := "Schedule: " + name
	if name == "" {
		title = "Schedule: " + jobID
	}
	prompt := job.Prompt
	s.mu.Unlock()

	sessionID, err := s.rt.CreateSession(fctx, title)
	if err == nil {
		s.mu.Lock()
		if _, ok := s.inflight[jobID]; ok {
			s.inflight[jobID] = sessionID
		}
		s.mu.Unlock()
		err = s.rt.MarkUnattended(fctx, sessionID)
	}
	if err == nil {
		err = s.rt.SendPrompt(fctx, sessionID, prompt)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	job = s.jobLocked(jobID)
	if err != nil {
		delete(s.inflight, jobID)
		if job != nil {
			s.recordLaunchFailureLocked(job, previousRun, err)
		}
		return
	}
	delete(s.retryAfter, jobID)
	if job == nil {
		delete(s.inflight, jobID)
		return
	}
	job.RecentFires = append(pruneFires(job.RecentFires, now), now)
	_ = s.persistLocked(job.ID)
}

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

func (s *Scheduler) noteSkip(jobID string, err error) {
	reason := "skipped: " + err.Error()
	s.mu.Lock()
	defer s.mu.Unlock()
	job := s.jobLocked(jobID)
	if job == nil || job.LastOutcome == reason {
		return
	}
	job.LastOutcome = reason
	_ = s.persistLocked(job.ID)
}

func (s *Scheduler) jobLocked(id string) *Job {
	for _, job := range s.file.Jobs {
		if job != nil && job.ID == id {
			return job
		}
	}
	return nil
}

func (s *Scheduler) persistLocked(jobID string) error {
	if err := SaveFile(s.path, &s.file, s.now()); err != nil {
		s.log.Warn("schedule: persist failed", "job", jobID, "err", err)
		return err
	}
	return nil
}

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
		if job.Enabled {
			job.DisabledReason = ""
		}
		job.RecentFires = pruneFires(job.RecentFires, now)
	}
	s.file = *file
	return nil
}
