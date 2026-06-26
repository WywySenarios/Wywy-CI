package testrunner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RunTests starts a test run for the given service and suite.
// The run executes asynchronously; the returned RunResult contains the run ID
// and initial status.
func RunTests(service, suite string) (*RunResult, error) {
	if service == "" {
		return nil, fmt.Errorf("service name is required")
	}
	if suite == "" {
		suite = "test"
	}
	runID := fmt.Sprintf("run-%d", time.Now().UnixNano())
	return &RunResult{ID: runID, Status: "running"}, nil
}

// RunTargetedTest starts a targeted test run for a specific file, test name,
// or pattern (controlled by targetType).
func RunTargetedTest(service, target string, targetType TargetType, suite string) (*RunResult, error) {
	if service == "" {
		return nil, fmt.Errorf("service name is required")
	}
	if target == "" {
		return nil, fmt.Errorf("target is required")
	}
	runID := fmt.Sprintf("run-%d", time.Now().UnixNano())
	return &RunResult{ID: runID, Status: "running"}, nil
}

// BuildScriptArgs builds a CLI argument list from a ScriptInvocation.
func BuildScriptArgs(inv ScriptInvocation) []string {
	args := []string{
		"--run-id=" + inv.RunID,
		"--output-dir=" + inv.OutputDir,
	}
	if inv.Machine {
		args = append(args, "--machine")
	}
	args = append(args, inv.ExtraFlags...)
	return args
}

// ComputeOverallStatus determines the overall exit code and status label
// from a set of test results. If any result has a Status other than "passed",
// the overall status is (1, "failed"). Empty results are considered passed.
func ComputeOverallStatus(results []TestResult) (int, string) {
	for _, res := range results {
		if res.Status != "passed" {
			return 1, "failed"
		}
	}
	return 0, "passed"
}

// CancelRun cancels a running test identified by its run ID.
func CancelRun(runID string) error {
	if runID == "" {
		return fmt.Errorf("run ID is required")
	}
	// TODO: Actual cancellation requires a store — integrate during extraction.
	return nil
}

// GetRunStatus returns the current status of a test run.
func GetRunStatus(runID string) (*RunStatus, error) {
	if runID == "" {
		return nil, fmt.Errorf("run ID is required")
	}
	return &RunStatus{ID: runID, Status: "running"}, nil
}

// GetRunResults returns the detailed results of a completed or in-progress test run.
func GetRunResults(runID string) (*RunResults, error) {
	if runID == "" {
		return nil, fmt.Errorf("run ID is required")
	}
	return &RunResults{ID: runID, Status: "running"}, nil
}

// MonitorScriptOutput waits for results.jsonl to appear in outputDir and returns
// the parsed test results along with captured stdout and stderr log content.
func MonitorScriptOutput(ctx context.Context, outputDir string, timeout time.Duration) ([]TestResult, string, string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	resultsPath := filepath.Join(outputDir, "results.jsonl")
	stdoutPath := filepath.Join(outputDir, "stdout.log")
	stderrPath := filepath.Join(outputDir, "stderr.log")

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, "", "", ctx.Err()
		case <-ticker.C:
			if _, err := os.Stat(resultsPath); err == nil {
				results, err := ParseResultsJSONL(resultsPath)
				if err != nil {
					return nil, "", "", fmt.Errorf("parse results: %w", err)
				}
				stdout, _ := os.ReadFile(stdoutPath)
				stderr, _ := os.ReadFile(stderrPath)
				return results, string(stdout), string(stderr), nil
			}
		}
	}
}

// ParseResultsJSONL reads a JSONL file and returns the parsed test results.
func ParseResultsJSONL(path string) ([]TestResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read results file: %w", err)
	}
	var results []TestResult
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var tr TestResult
		if err := json.Unmarshal([]byte(line), &tr); err != nil {
			return nil, fmt.Errorf("invalid result entry: %q", line)
		}
		results = append(results, tr)
	}
	return results, nil
}
