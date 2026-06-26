// Package testrunner provides the API for running tests across Wywy services.
// It is extracted from server/orchestrator to enable shared access from both
// the HTTP API server and the MCP server.
package testrunner

// RunResult wraps the result of starting a test run.
type RunResult struct {
	ID     string
	Status string
}

// TargetType controls how a targeted test is resolved.
type TargetType string

const (
	TargetFile     TargetType = "file"
	TargetTestName TargetType = "test_name"
	TargetPattern  TargetType = "pattern"
)

// Framework identifies a test framework's JSON output format.
type Framework string

const (
	FrameworkGoTestJSON     Framework = "go-test-json"
	FrameworkVitestJSON     Framework = "vitest-json"
	FrameworkPlaywrightJSON Framework = "playwright-json"
)

// ScriptInvocation defines the parameters for invoking a test script.
type ScriptInvocation struct {
	RunID      string
	OutputDir  string
	Machine    bool
	ExtraFlags []string
}

// TestResult holds the parsed results for a single test case.
type TestResult struct {
	Name     string
	Status   string
	Passed   int
	Failed   int
	Skipped  int
	Total    int
	Duration float64
}

// RunStatus holds the current status of a test run.
type RunStatus struct {
	ID              string
	Status          string
	RunningServices []string
}

// RunResults holds the detailed results of a completed or in-progress test run.
type RunResults struct {
	ID       string
	Status   string
	Services []ServiceResult
}

// ServiceResult holds the results for a single service within a run.
type ServiceResult struct {
	Name    string
	Status  string
	Passed  int
	Failed  int
	Skipped int
	LogRef  string
}
