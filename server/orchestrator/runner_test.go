package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"wywy-website/ci/server/store"
)

// fakeRunner returns a fixed exit code and output.
type fakeRunner struct {
	exitCode int
	output   string
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) (int, string, error) {
	return f.exitCode, f.output, nil
}

// serviceRunner returns different exit codes based on which service name
// appears in the command args.
type serviceRunner struct {
	codes  map[string]int
	output string
}

func (s *serviceRunner) Run(ctx context.Context, name string, args ...string) (int, string, error) {
	cmd := strings.Join(args, " ")
	for svc, code := range s.codes {
		if strings.Contains(cmd, svc) {
			return code, s.output, nil
		}
	}
	return 0, s.output, nil
}

// runPoller waits for a run to complete and returns the final run.
// It fails the test if the poll times out.
func runPoller(t *testing.T, st *store.Store, runID string) *store.Run {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var run *store.Run
	var err error
	for time.Now().Before(deadline) {
		run, err = st.GetRun(runID)
		if err != nil {
			t.Fatalf("GetRun(%q): %v", runID, err)
		}
		if run.Status != "running" && run.Status != "pending" {
			return run
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("run %q timed out with status %q", runID, run.Status)
	return nil
}

func TestRunLifecycleSuccess(t *testing.T) {
	s := newTestStore(t)

	r := NewRunner(s, &fakeRunner{exitCode: 0, output: "all tests passed"})

	ctx := context.Background()
	run, err := r.StartRun(ctx, []string{"agentic"}, "test")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if run == nil {
		t.Fatal("StartRun returned nil")
	}

	final := runPoller(t, s, run.ID)

	if final.Status != "passed" {
		t.Errorf("run Status: want passed, got %q", final.Status)
	}
	if final.FinishedAt == "" {
		t.Error("FinishedAt should be set")
	}

	// Check run_service.
	rs, err := s.GetRunService(run.ID, "agentic")
	if err != nil {
		t.Fatalf("GetRunService: %v", err)
	}
	if rs.Status != "passed" {
		t.Errorf("run_service Status: want passed, got %q", rs.Status)
	}
	if rs.ExitCode == nil || *rs.ExitCode != 0 {
		t.Errorf("run_service ExitCode: want 0, got %v", rs.ExitCode)
	}
}

func TestMultiServiceOneFails(t *testing.T) {
	s := newTestStore(t)

	runner := &serviceRunner{
		codes:  map[string]int{"agentic": 0, "cache": 1},
		output: "some output",
	}
	r := NewRunner(s, runner)

	ctx := context.Background()
	run, err := r.StartRun(ctx, []string{"agentic", "cache"}, "test")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if run == nil {
		t.Fatal("StartRun returned nil")
	}

	final := runPoller(t, s, run.ID)

	if final.Status != "failed" {
		t.Errorf("run Status: want failed, got %q", final.Status)
	}
	if final.FinishedAt == "" {
		t.Error("FinishedAt should be set")
	}

	// agentic passed.
	rs, err := s.GetRunService(run.ID, "agentic")
	if err != nil {
		t.Fatalf("GetRunService(agentic): %v", err)
	}
	if rs.Status != "passed" {
		t.Errorf("agentic Status: want passed, got %q", rs.Status)
	}
	if rs.ExitCode == nil || *rs.ExitCode != 0 {
		t.Errorf("agentic ExitCode: want 0, got %v", rs.ExitCode)
	}

	// cache failed.
	rs2, err := s.GetRunService(run.ID, "cache")
	if err != nil {
		t.Fatalf("GetRunService(cache): %v", err)
	}
	if rs2.Status != "failed" {
		t.Errorf("cache Status: want failed, got %q", rs2.Status)
	}
	if rs2.ExitCode == nil || *rs2.ExitCode != 1 {
		t.Errorf("cache ExitCode: want 1, got %v", rs2.ExitCode)
	}
}

// blockingRunner blocks until the context is cancelled.
type blockingRunner struct{}

func (b *blockingRunner) Run(ctx context.Context, name string, args ...string) (int, string, error) {
	<-ctx.Done()
	return -1, "", ctx.Err()
}

func TestRunCancellation(t *testing.T) {
	s := newTestStore(t)
	r := NewRunner(s, &blockingRunner{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	run, err := r.StartRun(ctx, []string{"agentic"}, "test")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Give the goroutine time to start and block.
	time.Sleep(50 * time.Millisecond)

	// Cancel the run.
	cancel()

	final := runPoller(t, s, run.ID)

	if final.Status != "cancelled" {
		t.Errorf("run Status: want cancelled, got %q", final.Status)
	}
	if final.FinishedAt == "" {
		t.Error("FinishedAt should be set")
	}

	// RunService should be cancelled.
	rs, err := s.GetRunService(run.ID, "agentic")
	if err != nil {
		t.Fatalf("GetRunService: %v", err)
	}
	if rs.Status != "cancelled" {
		t.Errorf("run_service Status: want cancelled, got %q", rs.Status)
	}
}

// newTestStore opens an in-memory store for orchestration tests.
func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestStartRunInvalidService(t *testing.T) {
	s := newTestStore(t)
	runner := NewRunnerWithServices(s, &fakeRunner{exitCode: 0},
		map[string]bool{"agentic": true, "cache": true})

	ctx := context.Background()
	_, err := runner.StartRun(ctx, []string{"nonexistent"}, "test")
	if err == nil {
		t.Fatal("StartRun: expected error for invalid service, got nil")
	}
}

// multiLineRunner returns different output depending on which service name
// appears in the command args, simulating real test output with mixed JSON
// and plain text lines.
type multiLineRunner struct {
	codes map[string]int // service → exit code
}

func (m *multiLineRunner) Run(ctx context.Context, name string, args ...string) (int, string, error) {
	cmd := strings.Join(args, " ")
	for svc, exitCode := range m.codes {
		if strings.Contains(cmd, svc) {
			return exitCode, m.outputFor(svc), nil
		}
	}
	return 0, "", nil
}

func (m *multiLineRunner) outputFor(service string) string {
	return `{"ts":"2026-06-13T10:00:00Z","service":"` + service + `","level":"INFO","msg":"test started"}
{"ts":"2026-06-13T10:00:01Z","service":"` + service + `","level":"INFO","msg":"running test suite"}
Running test_case_1...
{"ts":"2026-06-13T10:00:02Z","service":"` + service + `","level":"ERROR","msg":"test_case_1 failed: timeout"}
{"ts":"2026-06-13T10:00:03Z","service":"` + service + `","level":"INFO","msg":"test complete"}
`
}

func TestRunnerCapturesLogOutput(t *testing.T) {
	s := newTestStore(t)
	r := NewRunner(s, &multiLineRunner{codes: map[string]int{"agentic": 0}})

	ctx := context.Background()
	run, err := r.StartRun(ctx, []string{"agentic"}, "test")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	// Wait for completion.
	final := runPoller(t, s, run.ID)

	if final.Status != "passed" {
		t.Fatalf("run Status: want passed, got %q", final.Status)
	}

	// Query log entries for the run+service.
	entries, err := s.QueryLogEntries(run.ID, "agentic", store.LogQueryOpts{})
	if err != nil {
		t.Fatalf("QueryLogEntries: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("expected log entries in store, got none — Runner is not capturing output")
	}

	// Verify entry structure: 5 lines = 5 entries.
	if len(entries) != 5 {
		t.Fatalf("expected 5 log entries, got %d", len(entries))
	}

	// First entry: JSON-line with level INFO.
	if entries[0].Level != "INFO" {
		t.Errorf("entry[0] Level: want INFO, got %q", entries[0].Level)
	}
	if entries[0].Content != "test started" {
		t.Errorf("entry[0] Content: want %q, got %q", "test started", entries[0].Content)
	}
	if entries[0].ServiceName != "agentic" {
		t.Errorf("entry[0] ServiceName: want agentic, got %q", entries[0].ServiceName)
	}

	// Third entry: the plain text line should be RAW level.
	if entries[2].Level != "RAW" {
		t.Errorf("entry[2] Level: want RAW, got %q", entries[2].Level)
	}
	if entries[2].Content != "Running test_case_1..." {
		t.Errorf("entry[2] Content: want %q, got %q", "Running test_case_1...", entries[2].Content)
	}

	// Fourth entry: JSON-line with level ERROR.
	if entries[3].Level != "ERROR" {
		t.Errorf("entry[3] Level: want ERROR, got %q", entries[3].Level)
	}
	if entries[3].Content != "test_case_1 failed: timeout" {
		t.Errorf("entry[3] Content: want %q, got %q", "test_case_1 failed: timeout", entries[3].Content)
	}
}

// recordingBroadcaster records all Send and Done calls for verification.
type recordingBroadcaster struct {
	sends []LogMessage
	done  []string // recorded status values
}

func (r *recordingBroadcaster) Send(_ string, msg LogMessage) {
	r.sends = append(r.sends, msg)
}

func (r *recordingBroadcaster) Done(_ string, status string) {
	r.done = append(r.done, status)
}

func TestRunnerBroadcastsLogEntries(t *testing.T) {
	s := newTestStore(t)
	rec := &recordingBroadcaster{}
	r := NewRunner(s, &multiLineRunner{codes: map[string]int{"agentic": 0}})
	r.SetBroadcaster(rec)

	ctx := context.Background()
	run, err := r.StartRun(ctx, []string{"agentic"}, "test")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	final := runPoller(t, s, run.ID)
	if final.Status != "passed" {
		t.Fatalf("run Status: want passed, got %q", final.Status)
	}

	// Broadcaster.Send must have been called once per parsed log entry (5 lines).
	if len(rec.sends) == 0 {
		t.Fatal("broadcaster.Send was never called — Runner is not broadcasting")
	}
	if len(rec.sends) != 5 {
		t.Fatalf("expected 5 Send calls, got %d", len(rec.sends))
	}

	// First call: JSON line with INFO level.
	if rec.sends[0].Level != "INFO" {
		t.Errorf("send[0] Level: want INFO, got %q", rec.sends[0].Level)
	}
	if rec.sends[0].Content != "test started" {
		t.Errorf("send[0] Content: want %q, got %q", "test started", rec.sends[0].Content)
	}
	if rec.sends[0].ServiceName != "agentic" {
		t.Errorf("send[0] ServiceName: want agentic, got %q", rec.sends[0].ServiceName)
	}

	// Third call: the plain text line should be RAW level.
	if rec.sends[2].Level != "RAW" {
		t.Errorf("send[2] Level: want RAW, got %q", rec.sends[2].Level)
	}
	if rec.sends[2].Content != "Running test_case_1..." {
		t.Errorf("send[2] Content: want %q, got %q", "Running test_case_1...", rec.sends[2].Content)
	}

	// Fourth call: JSON line with ERROR level.
	if rec.sends[3].Level != "ERROR" {
		t.Errorf("send[3] Level: want ERROR, got %q", rec.sends[3].Level)
	}
	if rec.sends[3].Content != "test_case_1 failed: timeout" {
		t.Errorf("send[3] Content: want %q, got %q", "test_case_1 failed: timeout", rec.sends[3].Content)
	}

	// Broadcaster.Done must have been called exactly once with status "passed".
	if len(rec.done) == 0 {
		t.Fatal("broadcaster.Done was never called — Runner is not broadcasting completion")
	}
	if len(rec.done) != 1 {
		t.Fatalf("expected 1 Done call, got %d", len(rec.done))
	}
	if rec.done[0] != "passed" {
		t.Errorf("Done status: want %q, got %q", "passed", rec.done[0])
	}
}

func TestRunnerWritesLogOutputToFilesystem(t *testing.T) {
	logsDir := t.TempDir()

	s := newTestStore(t)
	r := NewRunner(s, &multiLineRunner{codes: map[string]int{"agentic": 0}})
	r.LogsDir = logsDir

	ctx := context.Background()
	run, err := r.StartRun(ctx, []string{"agentic"}, "test")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	final := runPoller(t, s, run.ID)
	if final.Status != "passed" {
		t.Fatalf("run Status: want passed, got %q", final.Status)
	}

	logPath := filepath.Join(logsDir, run.ID, "agentic.log")

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Fatalf("log file not found at %s — Runner is not writing log output to filesystem", logPath)
	}

	// Verify file contains the raw output.
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", logPath, err)
	}
	got := string(data)

	// The raw output should match what the fake runner returned.
	want := (&multiLineRunner{}).outputFor("agentic")
	if got != want {
		t.Errorf("log file content:\ngot:\n%s\nwant:\n%s", got, want)
	}
}
