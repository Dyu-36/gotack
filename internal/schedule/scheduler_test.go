package schedule

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// scheduler_test.go -- role: behavioural proofs for the runner: firing
// semantics, the unattended-mark-before-send order, duplicate and interval
// guards across restarts, failure policy, and SSE outcome bookkeeping.

// fakeRuntime records every seam call in order and lets each test script the
// behaviour of the desktop-side functions.
type fakeRuntime struct {
	mu         sync.Mutex
	calls      []string
	createErr  error
	markErr    error
	sendErr    error
	preflight  error
	sessionSeq int
	// sentPrompts maps session id -> prompt for assertions.
	sentPrompts map[string]string
}

func (f *fakeRuntime) record(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, name)
}

func (f *fakeRuntime) callNames() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.calls))
	copy(out, f.calls)
	return out
}

func (f *fakeRuntime) CreateSession(_ context.Context, title string) (string, error) {
	f.record("create")
	if f.createErr != nil {
		return "", f.createErr
	}
	f.sessionSeq++
	return fmt.Sprintf("sess-%d", f.sessionSeq), nil
}

func (f *fakeRuntime) MarkUnattended(_ context.Context, sessionID string) error {
	f.record("mark:" + sessionID)
	return f.markErr
}

func (f *fakeRuntime) SendPrompt(_ context.Context, sessionID, prompt string) (string, error) {
	f.record("send:" + sessionID)
	if f.sendErr != nil {
		return "", f.sendErr
	}
	if f.sentPrompts == nil {
		f.sentPrompts = make(map[string]string)
	}
	f.sentPrompts[sessionID] = prompt
	return "run-" + sessionID, nil
}

func (f *fakeRuntime) Preflight(context.Context) error {
	f.record("preflight")
	return f.preflight
}

func newTestScheduler(t *testing.T, rt Runtime, now time.Time) *Scheduler {
	t.Helper()
	path := filepath.Join(t.TempDir(), FileName)
	s := New(path, rt, slog.New(slog.DiscardHandler))
	s.now = func() time.Time { return now }
	return s
}

func testJob() *Job {
	return &Job{ID: "job1", Name: "Digest", Prompt: "summarise the workspace", Every: "10m", Enabled: true}
}

func loadStored(t *testing.T, s *Scheduler) *File {
	t.Helper()
	file, err := LoadFile(s.path)
	if err != nil {
		t.Fatalf("load stored schedule: %v", err)
	}
	return file
}

func TestFireMarksUnattendedBeforeSend(t *testing.T) {
	rt := &fakeRuntime{}
	s := newTestScheduler(t, Runtime{
		CreateSession:  rt.CreateSession,
		MarkUnattended: rt.MarkUnattended,
		SendPrompt:     rt.SendPrompt,
		Preflight:      rt.Preflight,
	}, base2026())
	s.file = File{Jobs: []*Job{testJob()}}
	s.engineReady = true

	s.evaluate(context.Background())

	want := []string{"preflight", "create", "mark:sess-1", "send:sess-1"}
	got := rt.callNames()
	if len(got) != len(want) {
		t.Fatalf("calls = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("call %d = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
	// Bookkeeping is persisted at claim time so it survives restarts.
	stored := loadStored(t, s)
	job := stored.Jobs[0]
	if job.LastRun == nil {
		t.Fatal("last run not persisted after firing")
	}
	if len(job.RecentFires) != 1 {
		t.Fatalf("recent fires not recorded: %v", job.RecentFires)
	}
	if s.inflightCount() != 1 {
		t.Fatalf("run must stay in flight until run_complete, got %d", s.inflightCount())
	}
}

func TestFailedMarkAbortsSend(t *testing.T) {
	rt := &fakeRuntime{markErr: errors.New("roster write failed")}
	s := newTestScheduler(t, Runtime{
		CreateSession:  rt.CreateSession,
		MarkUnattended: rt.MarkUnattended,
		SendPrompt:     rt.SendPrompt,
		Preflight:      rt.Preflight,
	}, base2026())
	s.file = File{Jobs: []*Job{testJob()}}
	s.engineReady = true

	s.evaluate(context.Background())

	for _, call := range rt.callNames() {
		if strings.HasPrefix(call, "send") {
			t.Fatalf("prompt must never be sent when the unattended mark fails, calls: %v", rt.callNames())
		}
	}
	if s.inflightCount() != 0 {
		t.Fatal("failed launch must not leave an in-flight entry")
	}
	job := s.file.Jobs[0]
	if job.ConsecutiveFailures != 1 {
		t.Fatalf("failed mark must count as a failure, got %d", job.ConsecutiveFailures)
	}
	if !strings.Contains(job.LastOutcome, "roster write failed") {
		t.Fatalf("outcome must record the failure reason, got %q", job.LastOutcome)
	}
	// The interval claim is rolled back so the policy can retry the firing.
	if job.LastRun != nil {
		t.Fatal("failed launch must roll back the last-run claim")
	}
}

func TestFailedSendCountsFailure(t *testing.T) {
	rt := &fakeRuntime{sendErr: errors.New("engine refused")}
	s := newTestScheduler(t, Runtime{
		CreateSession:  rt.CreateSession,
		MarkUnattended: rt.MarkUnattended,
		SendPrompt:     rt.SendPrompt,
		Preflight:      rt.Preflight,
	}, base2026())
	s.file = File{Jobs: []*Job{testJob()}}
	s.engineReady = true

	s.evaluate(context.Background())

	job := s.file.Jobs[0]
	if job.ConsecutiveFailures != 1 || !strings.Contains(job.LastOutcome, "engine refused") {
		t.Fatalf("send failure not recorded: %+v", job)
	}
}

func TestSameJobNeverFiresConcurrently(t *testing.T) {
	rt := &fakeRuntime{}
	s := newTestScheduler(t, Runtime{
		CreateSession:  rt.CreateSession,
		MarkUnattended: rt.MarkUnattended,
		SendPrompt:     rt.SendPrompt,
		Preflight:      rt.Preflight,
	}, base2026())
	s.file = File{Jobs: []*Job{testJob()}}
	s.engineReady = true

	s.evaluate(context.Background())
	// The previous run has not completed; the interval has elapsed anyway.
	s.evaluate(context.Background())

	sends := 0
	for _, call := range rt.callNames() {
		if strings.HasPrefix(call, "send") {
			sends++
		}
	}
	if sends != 1 {
		t.Fatalf("a job in flight must not fire again, sends = %d (%v)", sends, rt.callNames())
	}
}

func TestIntervalWindowBlocksRefire(t *testing.T) {
	rt := &fakeRuntime{}
	now := base2026()
	s := newTestScheduler(t, Runtime{
		CreateSession:  rt.CreateSession,
		MarkUnattended: rt.MarkUnattended,
		SendPrompt:     rt.SendPrompt,
		Preflight:      rt.Preflight,
	}, now)
	s.file = File{Jobs: []*Job{testJob()}}
	s.engineReady = true

	s.evaluate(context.Background())
	s.RecordOutcome("sess-1", "", false)
	// Still inside the 10m interval: nothing may fire.
	s.evaluate(context.Background())

	sends := 0
	for _, call := range rt.callNames() {
		if strings.HasPrefix(call, "send") {
			sends++
		}
	}
	if sends != 1 {
		t.Fatalf("refire inside the interval window, sends = %d", sends)
	}
}

func TestIntervalGuardSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	rt := Runtime{
		CreateSession:  (&fakeRuntime{}).CreateSession,
		MarkUnattended: (&fakeRuntime{}).MarkUnattended,
		SendPrompt:     (&fakeRuntime{}).SendPrompt,
		Preflight:      (&fakeRuntime{}).Preflight,
	}

	// First host run fires the job once.
	first := New(path, rt, slog.New(slog.DiscardHandler))
	first.now = base2026
	first.file = File{Jobs: []*Job{testJob()}}
	first.engineReady = true
	first.evaluate(context.Background())
	first.RecordOutcome("sess-1", "", false)

	// Restart: a fresh scheduler loads the persisted last-run and must not
	// re-fire inside the interval window.
	secondRT := &fakeRuntime{}
	second := New(path, Runtime{
		CreateSession:  secondRT.CreateSession,
		MarkUnattended: secondRT.MarkUnattended,
		SendPrompt:     secondRT.SendPrompt,
		Preflight:      secondRT.Preflight,
	}, slog.New(slog.DiscardHandler))
	second.now = base2026
	if err := second.load(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	second.engineReady = true
	second.evaluate(context.Background())

	for _, call := range secondRT.callNames() {
		if strings.HasPrefix(call, "send") {
			t.Fatalf("restarted host re-fired inside the interval window: %v", secondRT.callNames())
		}
	}
}

func TestBudgetCapsFiringsPerHour(t *testing.T) {
	rt := &fakeRuntime{}
	s := newTestScheduler(t, Runtime{
		CreateSession:  rt.CreateSession,
		MarkUnattended: rt.MarkUnattended,
		SendPrompt:     rt.SendPrompt,
		Preflight:      rt.Preflight,
	}, base2026())
	job := testJob()
	job.HourlyBudget = 1
	s.file = File{Jobs: []*Job{job}}
	s.engineReady = true

	// Three attempts inside one hour: the budget allows exactly one launch.
	for i := 0; i < 3; i++ {
		s.evaluate(context.Background())
		s.RecordOutcome(fmt.Sprintf("sess-%d", i+1), "", false)
	}

	sends := 0
	for _, call := range rt.callNames() {
		if strings.HasPrefix(call, "send") {
			sends++
		}
	}
	if sends != 1 {
		t.Fatalf("hourly budget 1 allowed %d firings: %v", sends, rt.callNames())
	}
}

func TestConsecutiveLaunchFailuresDisableJob(t *testing.T) {
	rt := &fakeRuntime{createErr: errors.New("engine gone")}
	s := newTestScheduler(t, Runtime{
		CreateSession:  rt.CreateSession,
		MarkUnattended: rt.MarkUnattended,
		SendPrompt:     rt.SendPrompt,
		Preflight:      rt.Preflight,
	}, base2026())
	s.retryDelay = 0 // let every tick retry immediately
	s.file = File{Jobs: []*Job{testJob()}}
	s.engineReady = true

	for i := 0; i < defaultFailureThreshold; i++ {
		s.evaluate(context.Background())
	}

	job := s.file.Jobs[0]
	if job.Enabled {
		t.Fatal("job must be disabled after the failure threshold")
	}
	if !strings.Contains(job.DisabledReason, fmt.Sprintf("%d consecutive failures", defaultFailureThreshold)) {
		t.Fatalf("disabled reason must name the threshold, got %q", job.DisabledReason)
	}
	// A disabled job never fires again on its own.
	rt.createErr = nil
	s.evaluate(context.Background())
	for _, call := range rt.callNames() {
		if strings.HasPrefix(call, "send") {
			t.Fatalf("disabled job fired again: %v", rt.callNames())
		}
	}
	// The persisted state carries the disablement across restarts.
	stored := loadStored(t, s)
	if stored.Jobs[0].Enabled || stored.Jobs[0].DisabledReason == "" {
		t.Fatalf("disablement not persisted: %+v", stored.Jobs[0])
	}
}

func TestRunOutcomesFromSSE(t *testing.T) {
	rt := &fakeRuntime{}
	s := newTestScheduler(t, Runtime{
		CreateSession:  rt.CreateSession,
		MarkUnattended: rt.MarkUnattended,
		SendPrompt:     rt.SendPrompt,
		Preflight:      rt.Preflight,
	}, base2026())
	s.file = File{Jobs: []*Job{testJob()}}
	s.engineReady = true

	// Run 1 fails inside the engine; the outcome arrives via run_complete.
	s.evaluate(context.Background())
	s.RecordOutcome("sess-1", "model overloaded", false)
	job := s.file.Jobs[0]
	if job.ConsecutiveFailures != 1 || !strings.Contains(job.LastOutcome, "model overloaded") {
		t.Fatalf("run failure not recorded from outcome: %+v", job)
	}
	if s.inflightCount() != 0 {
		t.Fatal("outcome must clear the in-flight entry")
	}

	// Run 2 succeeds and resets the failure streak. The clock advances past
	// the 10m interval so the job is due again.
	later := base2026().Add(11 * time.Minute)
	s.now = func() time.Time { return later }
	s.evaluate(context.Background())
	s.RecordOutcome("sess-2", "", false)
	if job.ConsecutiveFailures != 0 || job.LastOutcome != "complete" {
		t.Fatalf("successful outcome must reset failures: %+v", job)
	}

	// Outcomes for unknown sessions are ignored, never fatal.
	s.RecordOutcome("someone-elses-session", "boom", false)
}

func TestRunErrorsDisableAfterThreshold(t *testing.T) {
	rt := &fakeRuntime{}
	s := newTestScheduler(t, Runtime{
		CreateSession:  rt.CreateSession,
		MarkUnattended: rt.MarkUnattended,
		SendPrompt:     rt.SendPrompt,
		Preflight:      rt.Preflight,
	}, base2026())
	job := testJob()
	// Raise the budget so the failure threshold, not the budget, is what
	// this test exercises.
	job.HourlyBudget = defaultFailureThreshold + 1
	s.file = File{Jobs: []*Job{job}}
	s.engineReady = true

	for i := 0; i < defaultFailureThreshold; i++ {
		// Advance past the interval each round: the run launched, so the
		// claim stands and the next firing needs a fresh window.
		tick := base2026().Add(time.Duration(i+1) * 11 * time.Minute)
		s.now = func() time.Time { return tick }
		s.evaluate(context.Background())
		s.RecordOutcome(fmt.Sprintf("sess-%d", i+1), "agent error", false)
	}
	if job.Enabled {
		t.Fatalf("run-error failures must also disable the job after %d strikes: %+v", defaultFailureThreshold, job)
	}
}

func TestEngineNotReadyDefersWithoutFailures(t *testing.T) {
	rt := &fakeRuntime{}
	s := newTestScheduler(t, Runtime{
		CreateSession:  rt.CreateSession,
		MarkUnattended: rt.MarkUnattended,
		SendPrompt:     rt.SendPrompt,
		Preflight:      rt.Preflight,
	}, base2026())
	s.file = File{Jobs: []*Job{testJob()}}
	// engineReady stays false.

	s.evaluate(context.Background())

	if len(rt.callNames()) != 0 {
		t.Fatalf("no call may happen while the engine is down: %v", rt.callNames())
	}
	if job := s.file.Jobs[0]; job.ConsecutiveFailures != 0 || job.LastRun != nil {
		t.Fatalf("engine downtime must not count against the job: %+v", job)
	}

	// Readiness is pushed (SetEngineReady), and the overdue job fires then.
	s.SetEngineReady(true)
	sent := false
	for _, call := range rt.callNames() {
		if strings.HasPrefix(call, "send") {
			sent = true
		}
	}
	if !sent {
		t.Fatalf("engine becoming ready must re-evaluate due jobs: %v", rt.callNames())
	}
}

func TestPreflightSkipsWithoutCountingFailure(t *testing.T) {
	rt := &fakeRuntime{preflight: errors.New("no model configured")}
	s := newTestScheduler(t, Runtime{
		CreateSession:  rt.CreateSession,
		MarkUnattended: rt.MarkUnattended,
		SendPrompt:     rt.SendPrompt,
		Preflight:      rt.Preflight,
	}, base2026())
	s.file = File{Jobs: []*Job{testJob()}}
	s.engineReady = true

	s.evaluate(context.Background())

	for _, call := range rt.callNames() {
		if call == "create" {
			t.Fatalf("preflight failure must skip the run, calls: %v", rt.callNames())
		}
	}
	job := s.file.Jobs[0]
	if job.ConsecutiveFailures != 0 {
		t.Fatalf("a skipped run must not burn a failure strike: %+v", job)
	}
	if !strings.Contains(job.LastOutcome, "no model configured") {
		t.Fatalf("skip must be visible in the outcome, got %q", job.LastOutcome)
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}

func TestMalformedFileStartsWithoutJobs(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if err := writeFile(path, "{broken"); err != nil {
		t.Fatal(err)
	}
	rt := &fakeRuntime{}
	s := New(path, Runtime{
		CreateSession:  rt.CreateSession,
		MarkUnattended: rt.MarkUnattended,
		SendPrompt:     rt.SendPrompt,
		Preflight:      rt.Preflight,
	}, slog.New(slog.DiscardHandler))
	if err := s.load(); err == nil {
		t.Fatal("load must report the malformed file")
	}
	s.engineReady = true
	s.evaluate(context.Background())
	if len(rt.callNames()) != 0 {
		t.Fatalf("no job may run from an unparsed file: %v", rt.callNames())
	}
}
