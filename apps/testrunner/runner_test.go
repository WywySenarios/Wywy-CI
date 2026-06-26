package testrunner

import (
	"testing"
)

// TestRunTests verifies that RunTests starts a test run and returns a valid
// RunResult with a non-empty ID and initial status "running".
func TestRunTests(t *testing.T) {
	result, err := RunTests("ci", "test")
	if err != nil {
		t.Fatalf("RunTests returned error: %v", err)
	}
	if result == nil {
		t.Fatal("RunTests returned nil result")
	}
	if result.ID == "" {
		t.Error("RunTests returned empty run ID")
	}
	if result.Status != "running" {
		t.Errorf("RunTests status: want running, got %s", result.Status)
	}
}

// TestRunTargetedTest verifies that RunTargetedTest starts a targeted test
// run (by file, test name, or pattern) and returns a valid RunResult.
func TestRunTargetedTest(t *testing.T) {
	result, err := RunTargetedTest("ci", "TestFoo", TargetTestName, "test")
	if err != nil {
		t.Fatalf("RunTargetedTest returned error: %v", err)
	}
	if result == nil {
		t.Fatal("RunTargetedTest returned nil result")
	}
	if result.ID == "" {
		t.Error("RunTargetedTest returned empty run ID")
	}
	if result.Status != "running" {
		t.Errorf("RunTargetedTest status: want running, got %s", result.Status)
	}
}

// TestBuildScriptArgs verifies that BuildScriptArgs produces the correct CLI
// argument list from a ScriptInvocation.
func TestBuildScriptArgs(t *testing.T) {
	inv := ScriptInvocation{
		RunID:      "run-abc",
		OutputDir:  "/tmp/runs/run-abc",
		Machine:    true,
		ExtraFlags: []string{"--verbose"},
	}

	args := BuildScriptArgs(inv)

	expected := []string{"--run-id=run-abc", "--output-dir=/tmp/runs/run-abc", "--machine", "--verbose"}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, want := range expected {
		if args[i] != want {
			t.Errorf("arg[%d]: want %q, got %q", i, want, args[i])
		}
	}
}

// TestCancelRun verifies that CancelRun accepts a run ID and returns without
// error, cancelling the in-flight run.
func TestCancelRun(t *testing.T) {
	err := CancelRun("run-12345")
	if err != nil {
		t.Fatalf("CancelRun returned error: %v", err)
	}
}
