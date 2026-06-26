package testrunner

import (
	"encoding/json"
	"fmt"
	"strings"
)

// goTestEvent represents a single event line from `go test -json`.
type goTestEvent struct {
	Package string `json:"Package"`
	Test    string `json:"Test"`
	Action  string `json:"Action"`
}

// vitestReport represents the top-level Vitest JSON reporter output.
type vitestReport struct {
	TestResults []vitestFileResult `json:"testResults"`
}

// vitestFileResult represents results for a single test file.
type vitestFileResult struct {
	AssertionResults []vitestAssertion `json:"assertionResults"`
}

// vitestAssertion represents a single assertion result from Vitest.
type vitestAssertion struct {
	FullName string `json:"fullName"`
	Status   string `json:"status"`
}

// playwrightReport represents the top-level Playwright JSON reporter output.
type playwrightReport struct {
	Stats  playwrightStats   `json:"stats"`
	Suites []playwrightSuite `json:"suites"`
}

// playwrightStats holds summary statistics from Playwright.
type playwrightStats struct {
	Expected   int `json:"expected"`
	Unexpected int `json:"unexpected"`
	Skipped    int `json:"skipped"`
	Flaky      int `json:"flaky"`
}

// playwrightSuite represents a test suite in Playwright output.
type playwrightSuite struct {
	Specs []playwrightSpec `json:"specs"`
}

// playwrightSpec represents a single test spec in Playwright output.
type playwrightSpec struct {
	Title string `json:"title"`
	OK    bool   `json:"ok"`
}

// ParseFrameworkOutput parses JSON-formatted output from a test framework into
// structured test results. Supported frameworks are FrameworkGoTestJSON,
// FrameworkVitestJSON, and FrameworkPlaywrightJSON.
func ParseFrameworkOutput(output string, framework Framework) ([]TestResult, error) {
	switch framework {
	case FrameworkGoTestJSON:
		return parseGoTestJSON(output)
	case FrameworkVitestJSON:
		return parseVitestJSON(output)
	case FrameworkPlaywrightJSON:
		return parsePlaywrightJSON(output)
	default:
		return nil, fmt.Errorf("unknown framework: %s", framework)
	}
}

// parseGoTestJSON parses `go test -json` NDJSON output into TestResult entries.
// Each JSON line with a "pass", "fail", or "skip" action produces one entry.
func parseGoTestJSON(output string) ([]TestResult, error) {
	var results []TestResult
	for _, raw := range strings.Split(strings.TrimSpace(output), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		var evt goTestEvent
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			return nil, fmt.Errorf("invalid go test event: %w", err)
		}
		// Skip package-level action events (no Test field).
		if evt.Test == "" {
			continue
		}
		var status string
		switch evt.Action {
		case "pass":
			status = "passed"
		case "fail":
			status = "failed"
		case "skip":
			status = "skipped"
		default:
			continue
		}
		results = append(results, TestResult{Name: evt.Test, Status: status})
	}
	return results, nil
}

// parseVitestJSON parses Vitest JSON reporter output into TestResult entries.
func parseVitestJSON(output string) ([]TestResult, error) {
	var report vitestReport
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		return nil, fmt.Errorf("invalid vitest JSON: %w", err)
	}
	var results []TestResult
	for _, file := range report.TestResults {
		for _, a := range file.AssertionResults {
			status := a.Status
			if status == "passed" || status == "failed" || status == "skipped" {
				results = append(results, TestResult{Name: a.FullName, Status: status})
			}
		}
	}
	return results, nil
}

// parsePlaywrightJSON parses Playwright JSON reporter output into TestResult entries.
func parsePlaywrightJSON(output string) ([]TestResult, error) {
	var report playwrightReport
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		return nil, fmt.Errorf("invalid playwright JSON: %w", err)
	}
	var results []TestResult
	for _, suite := range report.Suites {
		for _, spec := range suite.Specs {
			status := "failed"
			if spec.OK {
				status = "passed"
			}
			results = append(results, TestResult{Name: spec.Title, Status: status})
		}
	}
	return results, nil
}
