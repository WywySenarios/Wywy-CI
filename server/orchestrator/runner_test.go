package orchestrator

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"wywy-ci/apps/testrunner"
	"wywy-ci/server/store"
)

// fakeRunner returns a fixed exit code and output.
type fakeRunner struct {
	exitCode int
	output   string
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) (int, string, error) {
	return f.exitCode, f.output, nil
}

// recordingCmdRunner records the command passed to Run and returns fixed output.
type recordingCmdRunner struct {
	Cmd    string // last command passed to Run
	Output string
}

func (r *recordingCmdRunner) Run(ctx context.Context, name string, args ...string) (int, string, error) {
	r.Cmd = name
	for _, a := range args {
		r.Cmd += " " + a
	}
	return 0, r.Output, nil
}

// recordingDetachedRunner implements both CommandRunner and DetachedCommandRunner.
// Run returns a fixed result (old path). StartDetached parses --output-dir from
// args and writes a results.jsonl with a failure to prove the new path was taken.
type recordingDetachedRunner struct{}

func (r *recordingDetachedRunner) Run(ctx context.Context, name string, args ...string) (int, string, error) {
	return 0, "old path output", nil
}

func (r *recordingDetachedRunner) StartDetached(ctx context.Context, outputDir, name string, args ...string) (*exec.Cmd, error) {
	os.MkdirAll(outputDir, 0755)
	os.WriteFile(filepath.Join(outputDir, "stdout.log"), []byte("script ran\n"), 0644)
	os.WriteFile(filepath.Join(outputDir, "stderr.log"), []byte(""), 0644)
	os.WriteFile(filepath.Join(outputDir, "results.jsonl"), []byte(`{"name":"t1","status":"failed"}`+"\n"), 0644)

	cmd := exec.CommandContext(ctx, "sh", "-c", "exit 0")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Start()
	return cmd, nil
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

func TestRunLifecycleWithDetachedRunner(t *testing.T) {
	s := newTestStore(t)
	runsDir := t.TempDir()

	resolver := NewServiceScriptResolver(
		map[string]string{"agentic": "Wywy-Agentic"},
		"/usr/local/Wywy-Website",
	)

	dr := &recordingDetachedRunner{}
	r := NewRunner(s, dr)
	r.RunsDir = runsDir
	r.SetResolver(resolver)

	ctx := context.Background()
	run, err := r.StartRun(ctx, []string{"agentic"}, "test")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	final := runPoller(t, s, run.ID)

	// The results.jsonl has a failed test => service should be "failed".
	rs, err := s.GetRunService(run.ID, "agentic")
	if err != nil {
		t.Fatalf("GetRunService: %v", err)
	}
	if rs.Status != "failed" {
		t.Errorf("run_service Status: want failed, got %q", rs.Status)
	}
	if final.Status != "failed" {
		t.Errorf("run Status: want failed, got %q", final.Status)
	}
}

func TestRunFailsWithoutResolver(t *testing.T) {
	s := newTestStore(t)
	r := NewRunner(s, &fakeRunner{exitCode: 0, output: ""})

	ctx := context.Background()
	run, err := r.StartRun(ctx, []string{"agentic"}, "test")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	final := runPoller(t, s, run.ID)
	if final.Status != "failed" {
		t.Errorf("run Status: want failed (no resolver configured), got %q", final.Status)
	}

	// RunService should be failed with a non-zero exit code.
	rs, err := s.GetRunService(run.ID, "agentic")
	if err != nil {
		t.Fatalf("GetRunService: %v", err)
	}
	if rs.Status != "failed" {
		t.Errorf("run_service Status: want failed, got %q", rs.Status)
	}
	if rs.ExitCode == nil || *rs.ExitCode == 0 {
		t.Errorf("run_service ExitCode: want non-zero, got %v", rs.ExitCode)
	}
}

func TestDetectOrphanedRuns(t *testing.T) {
	s := newTestStore(t)

	// Pre-populate store with a "running" run that has no active process.
	orphan := &store.Run{
		ID:        "run-orphan-1",
		CreatedAt: "2000-01-01T00:00:00Z",
		Status:    "running",
	}
	if err := s.CreateRun(orphan); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	rs := &store.RunService{
		RunID:       "run-orphan-1",
		ServiceName: "agentic",
		Suite:       "test",
		Status:      "running",
	}
	if err := s.CreateRunService(rs); err != nil {
		t.Fatalf("CreateRunService: %v", err)
	}

	r := NewRunner(s, &fakeRunner{exitCode: 0, output: ""})
	r.DetectOrphanedRuns()

	// Verify run was marked as failed.
	updated, err := s.GetRun("run-orphan-1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if updated.Status != "failed" {
		t.Errorf("run Status: want failed, got %q", updated.Status)
	}
	if updated.FinishedAt == "" {
		t.Error("FinishedAt should be set")
	}

	// Verify RunService was updated.
	updatedRS, err := s.GetRunService("run-orphan-1", "agentic")
	if err != nil {
		t.Fatalf("GetRunService: %v", err)
	}
	if updatedRS.Status != "failed" {
		t.Errorf("run_service Status: want failed, got %q", updatedRS.Status)
	}
}

// TestDetachedRunnerStoresLogEntries verifies that test output (stdout/stderr)
// from a detached script is persisted as log entries in the store.
func TestDetachedRunnerStoresLogEntries(t *testing.T) {
	s := newTestStore(t)
	runsDir := t.TempDir()

	resolver := NewServiceScriptResolver(
		map[string]string{"agentic": "Wywy-Agentic"},
		"/usr/local/Wywy-Website",
	)

	dr := &recordingDetachedRunner{}
	r := NewRunner(s, dr)
	r.RunsDir = runsDir
	r.SetResolver(resolver)

	ctx := context.Background()
	run, err := r.StartRun(ctx, []string{"agentic"}, "test")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	final := runPoller(t, s, run.ID)
	if final.Status != "failed" {
		t.Errorf("run Status: want failed, got %q", final.Status)
	}

	// The recordingDetachedRunner writes "script ran\n" to stdout.log.
	// These log entries should now be persisted in the store.
	logs, err := s.QueryAllLogEntries(run.ID, store.LogQueryOpts{})
	if err != nil {
		t.Fatalf("QueryAllLogEntries: %v", err)
	}

	if len(logs) == 0 {
		t.Fatal("expected log entries from detached test output, got none")
	}

	foundStdout := false
	for _, e := range logs {
		if e.Content == "script ran" || e.Content == "script ran\n" {
			foundStdout = true
			break
		}
	}
	if !foundStdout {
		t.Errorf("expected log entry containing stdout content %q; got: %v", "script ran", logs)
	}
}

// TestRunnerWithDefaultRunnerTakesDetachedPath verifies that when a Runner is
// configured with DefaultRunner, a resolver, and RunsDir, it takes the detached
// execution path instead of falling through to the "no execution path" error.
// It uses a real script that produces a passed results.jsonl immediately.
func TestRunnerWithDefaultRunnerTakesDetachedPath(t *testing.T) {
	s := newTestStore(t)
	runsDir := t.TempDir()

	// Create a real test script that the resolver will find.
	repoBase := t.TempDir()
	scriptPath := filepath.Join(repoBase, "Wywy-CI", "scripts", "tests", "test.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0755); err != nil {
		t.Fatal(err)
	}

	// The script receives --run-id=... and --output-dir=... from BuildScriptArgs.
	// It writes results.jsonl with a passed entry and exits.
	scriptContent := `#!/bin/sh
for arg in "$@"; do
  case "$arg" in
    --output-dir=*) output_dir="${arg#*=}" ;;
  esac
done
echo '{"name":"t1","status":"passed"}' > "$output_dir/results.jsonl"
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatal(err)
	}

	resolver := NewServiceScriptResolver(
		map[string]string{"ci": "Wywy-CI"},
		repoBase,
	)

	r := NewRunner(s, DefaultRunner)
	r.RunsDir = runsDir
	r.SetResolver(resolver)

	ctx := context.Background()
	run, err := r.StartRun(ctx, []string{"ci"}, "test")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	final := runPoller(t, s, run.ID)

	if final.Status != "passed" {
		t.Errorf("run Status: want passed, got %q", final.Status)
	}

	// Confirm the "no execution path configured" error was NOT logged.
	logs, err := s.QueryAllLogEntries(run.ID, store.LogQueryOpts{
		Search: "no execution path configured",
	})
	if err != nil {
		t.Fatalf("QueryAllLogEntries: %v", err)
	}
	if len(logs) > 0 {
		t.Errorf("found %d 'no execution path configured' error(s): %v", len(logs), logs[0].Content)
	}
}

// TestDefaultRunnerCapturesStdoutAndStderrToLogEntries verifies that the real
// DefaultRunner (DetachedRunner) captures the spawned script's stdout and stderr
// to stdout.log / stderr.log on disk and persists them as log entries in the store.
// This proves the API-level output capture contract — not just the mock path.
func TestDefaultRunnerCapturesStdoutAndStderrToLogEntries(t *testing.T) {
	s := newTestStore(t)
	runsDir := t.TempDir()
	repoBase := t.TempDir()

	// Create a real test script that produces stdout and stderr output.
	scriptPath := filepath.Join(repoBase, "Wywy-CI", "scripts", "tests", "test.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0755); err != nil {
		t.Fatal(err)
	}

	// The script writes to both stdout and stderr, then writes results.jsonl.
	scriptContent := `#!/bin/sh
echo "stdout hello"
echo "stderr world" >&2
for arg in "$@"; do
  case "$arg" in
    --output-dir=*) output_dir="${arg#*=}" ;;
  esac
done
echo '{"name":"t1","status":"passed"}' > "$output_dir/results.jsonl"
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatal(err)
	}

	resolver := NewServiceScriptResolver(
		map[string]string{"ci": "Wywy-CI"},
		repoBase,
	)

	r := NewRunner(s, DefaultRunner)
	r.RunsDir = runsDir
	r.SetResolver(resolver)

	ctx := context.Background()
	run, err := r.StartRun(ctx, []string{"ci"}, "test")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	final := runPoller(t, s, run.ID)
	if final.Status != "passed" {
		t.Errorf("run Status: want passed, got %q", final.Status)
	}

	// 1. On-disk stdout.log must exist and contain the script's stdout.
	stdoutPath := filepath.Join(runsDir, run.ID, "ci", "stdout.log")
	stdoutData, err := os.ReadFile(stdoutPath)
	if err != nil {
		t.Fatalf("failed to read stdout.log: %v", err)
	}
	if !strings.Contains(string(stdoutData), "stdout hello") {
		t.Errorf("stdout.log missing script output: got %q", string(stdoutData))
	}

	// 2. On-disk stderr.log must exist and contain the script's stderr.
	stderrPath := filepath.Join(runsDir, run.ID, "ci", "stderr.log")
	stderrData, err := os.ReadFile(stderrPath)
	if err != nil {
		t.Fatalf("failed to read stderr.log: %v", err)
	}
	if !strings.Contains(string(stderrData), "stderr world") {
		t.Errorf("stderr.log missing script output: got %q", string(stderrData))
	}

	// 3. Log entries in the store must include both stdout and stderr content.
	logs, err := s.QueryAllLogEntries(run.ID, store.LogQueryOpts{})
	if err != nil {
		t.Fatalf("QueryAllLogEntries: %v", err)
	}

	foundStdout := false
	foundStderr := false
	for _, e := range logs {
		if strings.Contains(e.Content, "stdout hello") {
			foundStdout = true
		}
		if strings.Contains(e.Content, "stderr world") {
			foundStderr = true
		}
	}
	if !foundStdout {
		t.Errorf("expected log entry containing %q; got %d logs: %v", "stdout hello", len(logs), logs)
	}
	if !foundStderr {
		t.Errorf("expected log entry containing %q; got %d logs: %v", "stderr world", len(logs), logs)
	}
}

// TestRunnerLogsMalformedJSONLError verifies that when a test script writes
// malformed results.jsonl (e.g. from a shell quoting bug), the parse error is
// captured as a log entry so the user can diagnose the issue.
func TestRunnerLogsMalformedJSONLError(t *testing.T) {
	s := newTestStore(t)
	runsDir := t.TempDir()

	// Create a script with the classic double-quote quoting bug:
	//   echo "{"name":"compliance","status":"passed"}"
	// Shell interprets inner " as delimiters, writing: {name:compliance,status:passed}
	repoBase := t.TempDir()
	scriptPath := filepath.Join(repoBase, "Wywy-Agentic", "scripts", "tests", "test.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0755); err != nil {
		t.Fatal(err)
	}

	scriptContent := `#!/bin/sh
output_dir=""
for arg in "$@"; do
  case "$arg" in
    --output-dir=*) output_dir="${arg#*=}" ;;
  esac
done
if [ -n "$output_dir" ]; then
  echo "{"name":"compliance","status":"passed"}" > "$output_dir/results.jsonl"
fi
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatal(err)
	}

	resolver := NewServiceScriptResolver(
		map[string]string{"agentic": "Wywy-Agentic"},
		repoBase,
	)

	r := NewRunner(s, DefaultRunner)
	r.RunsDir = runsDir
	r.SetResolver(resolver)

	ctx := context.Background()
	run, err := r.StartRun(ctx, []string{"agentic"}, "test")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	final := runPoller(t, s, run.ID)
	if final.Status != "failed" {
		t.Errorf("run Status: want failed, got %q", final.Status)
	}

	// The parse error describing the malformed JSON must be in the log entries.
	logs, err := s.QueryAllLogEntries(run.ID, store.LogQueryOpts{})
	if err != nil {
		t.Fatalf("QueryAllLogEntries: %v", err)
	}

	found := false
	for _, e := range logs {
		if strings.Contains(e.Content, "invalid result entry") ||
			strings.Contains(e.Content, "{name:compliance") ||
			strings.Contains(e.Content, "malformed") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected log entry containing the parse error detail; got %d entries:\n", len(logs))
		for _, e := range logs {
			t.Errorf("  [%s] %s", e.Level, e.Content)
		}
	}
}

// TestStartRunEmptyService verifies that StartRun rejects empty service names
// by delegating to testrunner.RunTests for validation. When validServices is
// nil (default NewRunner), StartRun must still catch an empty service name.
func TestStartRunEmptyService(t *testing.T) {
	s := newTestStore(t)
	r := NewRunner(s, &fakeRunner{exitCode: 0})

	ctx := context.Background()
	_, err := r.StartRun(ctx, []string{""}, "test")
	if err == nil {
		t.Fatal("StartRun with empty service: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "service") {
		t.Errorf("StartRun with empty service: error should mention 'service', got: %v", err)
	}
}

// TestRunnerRunTargetedTest verifies that Runner.RunTargetedTest delegates to
// testrunner.RunTargetedTest for validation and run ID generation.
func TestRunnerRunTargetedTest(t *testing.T) {
	s := newTestStore(t)
	r := NewRunner(s, &fakeRunner{exitCode: 0})

	// Valid inputs — must delegate to testrunner and return a run ID.
	id, err := r.RunTargetedTest("agentic", "test_auth.go", testrunner.TargetFile, "test")
	if err != nil {
		t.Fatalf("RunTargetedTest(agentic, test_auth.go, file, test): %v", err)
	}
	if id == "" {
		t.Error("RunTargetedTest: expected non-empty run ID")
	}

	// Empty service — testrunner validation must reject it.
	_, err = r.RunTargetedTest("", "test_auth.go", testrunner.TargetFile, "test")
	if err == nil {
		t.Error("RunTargetedTest with empty service: expected error, got nil")
	}

	// Empty target — testrunner validation must reject it.
	_, err = r.RunTargetedTest("agentic", "", testrunner.TargetFile, "test")
	if err == nil {
		t.Error("RunTargetedTest with empty target: expected error, got nil")
	}
}

// TestRunnerComputeOverallStatus verifies that Runner.ComputeOverallStatus
// delegates to testrunner to determine pass/fail from a set of results.
func TestRunnerComputeOverallStatus(t *testing.T) {
	s := newTestStore(t)
	r := NewRunner(s, &fakeRunner{exitCode: 0})

	// All passed — must return (0, "passed").
	code, status := r.ComputeOverallStatus([]store.ResultEntry{
		{Name: "t1", Status: "passed"},
		{Name: "t2", Status: "passed"},
	})
	if code != 0 {
		t.Errorf("all passed: want exit code 0, got %d", code)
	}
	if status != "passed" {
		t.Errorf("all passed: want status 'passed', got %q", status)
	}

	// Any failed — must return (1, "failed").
	code, status = r.ComputeOverallStatus([]store.ResultEntry{
		{Name: "t1", Status: "passed"},
		{Name: "t2", Status: "failed"},
	})
	if code != 1 {
		t.Errorf("any failed: want exit code 1, got %d", code)
	}
	if status != "failed" {
		t.Errorf("any failed: want status 'failed', got %q", status)
	}

	// Empty results — must return (0, "passed").
	code, status = r.ComputeOverallStatus(nil)
	if code != 0 {
		t.Errorf("empty: want exit code 0, got %d", code)
	}
	if status != "passed" {
		t.Errorf("empty: want status 'passed', got %q", status)
	}
}

// TestRunnerMonitorScriptOutput verifies that Runner.MonitorScriptOutput
// monitors a directory for results.jsonl, parses the test results, and
// returns them along with captured stdout and stderr from log files.
func TestRunnerMonitorScriptOutput(t *testing.T) {
	outputDir := t.TempDir()

	// Write results.jsonl, stdout.log, and stderr.log before calling so the
	// monitor finds them on the first poll tick.
	resultsContent := `{"name":"alpha","status":"passed","passed":1,"failed":0,"skipped":0,"total":1}
{"name":"beta","status":"failed","passed":0,"failed":1,"skipped":0,"total":1}`
	if err := os.WriteFile(filepath.Join(outputDir, "results.jsonl"), []byte(resultsContent+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "stdout.log"), []byte("line from stdout\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "stderr.log"), []byte("line from stderr\n"), 0644); err != nil {
		t.Fatal(err)
	}

	r := &Runner{}
	ctx := context.Background()

	results, stdout, stderr, err := r.MonitorScriptOutput(ctx, outputDir, 5*time.Second)
	if err != nil {
		t.Fatalf("MonitorScriptOutput: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Name != "alpha" || results[0].Status != "passed" {
		t.Errorf("result[0]: want Name=alpha Status=passed, got Name=%s Status=%s", results[0].Name, results[0].Status)
	}
	if results[1].Name != "beta" || results[1].Status != "failed" {
		t.Errorf("result[1]: want Name=beta Status=failed, got Name=%s Status=%s", results[1].Name, results[1].Status)
	}
	if stdout != "line from stdout\n" {
		t.Errorf("stdout: want %q, got %q", "line from stdout\n", stdout)
	}
	if stderr != "line from stderr\n" {
		t.Errorf("stderr: want %q, got %q", "line from stderr\n", stderr)
	}
}

// TestParseResultsJSONL verifies that testrunner exports a ParseResultsJSONL
// function so the orchestrator can parse results.jsonl files without importing
// the store package (e.g., from DetectOrphanedRuns).
func TestParseResultsJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "results.jsonl")
	content := `{"name":"alpha","status":"passed","passed":1,"failed":0,"skipped":0,"total":1}
{"name":"beta","status":"failed","passed":0,"failed":1,"skipped":0,"total":1}`
	if err := os.WriteFile(path, []byte(content+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := testrunner.ParseResultsJSONL(path)
	if err != nil {
		t.Fatalf("ParseResultsJSONL: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Name != "alpha" || results[0].Status != "passed" {
		t.Errorf("result[0]: want Name=alpha Status=passed, got Name=%s Status=%s", results[0].Name, results[0].Status)
	}
	if results[1].Name != "beta" || results[1].Status != "failed" {
		t.Errorf("result[1]: want Name=beta Status=failed, got Name=%s Status=%s", results[1].Name, results[1].Status)
	}
}

// TestRunnerCancelRun verifies that Runner.CancelRun delegates to
// testrunner.CancelRun for validation. A valid run ID returns nil;
// an empty run ID returns an error.
func TestRunnerCancelRun(t *testing.T) {
	s := newTestStore(t)
	r := NewRunner(s, &fakeRunner{exitCode: 0})

	// Valid run ID — must delegate to testrunner and return nil.
	err := r.CancelRun("run-valid-id")
	if err != nil {
		t.Fatalf("CancelRun('run-valid-id'): %v", err)
	}

	// Empty run ID — testrunner validation must reject it.
	err = r.CancelRun("")
	if err == nil {
		t.Error("CancelRun(''): expected error for empty run ID, got nil")
	}
}

