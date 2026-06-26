package testrunner

import (
	"testing"
)

// TestParseFrameworkOutputGoTestJSON verifies that ParseFrameworkOutput parses
// Go test -json output into structured TestResult entries.
func TestParseFrameworkOutputGoTestJSON(t *testing.T) {
	output := `{"Package":"wywy-ci/server/store","Test":"TestFoo","Action":"pass"}
{"Package":"wywy-ci/server/store","Test":"TestBar","Action":"fail"}
`
	results, err := ParseFrameworkOutput(output, FrameworkGoTestJSON)
	if err != nil {
		t.Fatalf("ParseFrameworkOutput(go-test-json): %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Name != "TestFoo" || results[0].Status != "passed" {
		t.Errorf("result[0]: want TestFoo/passed, got %s/%s", results[0].Name, results[0].Status)
	}
	if results[1].Name != "TestBar" || results[1].Status != "failed" {
		t.Errorf("result[1]: want TestBar/failed, got %s/%s", results[1].Name, results[1].Status)
	}
}

// TestParseFrameworkOutputVitestJSON verifies that ParseFrameworkOutput parses
// Vitest JSON reporter output into structured TestResult entries with counts.
func TestParseFrameworkOutputVitestJSON(t *testing.T) {
	output := `{
  "success": false,
  "numTotalTests": 3,
  "numPassedTests": 2,
  "numFailedTests": 1,
  "numPendingTests": 0,
  "testResults": [
    {
      "name": "components/Button.test.tsx",
      "assertionResults": [
        {"fullName": "Button renders", "status": "passed"},
        {"fullName": "Button clicks", "status": "passed"},
        {"fullName": "Button disabled", "status": "failed"}
      ]
    }
  ]
}
`
	results, err := ParseFrameworkOutput(output, FrameworkVitestJSON)
	if err != nil {
		t.Fatalf("ParseFrameworkOutput(vitest-json): %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
}

// TestParseFrameworkOutputPlaywrightJSON verifies that ParseFrameworkOutput
// parses Playwright JSON reporter output into structured TestResult entries.
func TestParseFrameworkOutputPlaywrightJSON(t *testing.T) {
	output := `{
  "stats": {
    "expected": 4,
    "unexpected": 1,
    "flaky": 0,
    "skipped": 1
  },
  "suites": [
    {
      "title": "homepage",
      "specs": [
        {"title": "loads", "ok": true},
        {"title": "shows content", "ok": true},
        {"title": "handles error", "ok": false}
      ]
    }
  ]
}
`
	results, err := ParseFrameworkOutput(output, FrameworkPlaywrightJSON)
	if err != nil {
		t.Fatalf("ParseFrameworkOutput(playwright-json): %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
}


