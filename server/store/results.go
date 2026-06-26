package store

import (
	"wywy-website/ci/apps/testrunner"
)

// ResultEntry represents a single test result from a results.jsonl file.
type ResultEntry struct {
	Name     string  `json:"name"`
	Status   string  `json:"status"`
	Passed   int     `json:"passed"`
	Failed   int     `json:"failed"`
	Skipped  int     `json:"skipped"`
	Total    int     `json:"total"`
	Duration float64 `json:"duration"`
}

// ParseResultsJSONL reads a results.jsonl file and returns the parsed entries.
func ParseResultsJSONL(path string) ([]ResultEntry, error) {
	results, err := testrunner.ParseResultsJSONL(path)
	if err != nil {
		return nil, err
	}
	return TestResultsToEntries(results), nil
}

// TestResultsToEntries converts testrunner TestResult values to store ResultEntry values.
func TestResultsToEntries(results []testrunner.TestResult) []ResultEntry {
	entries := make([]ResultEntry, len(results))
	for i, tr := range results {
		entries[i] = ResultEntry{
			Name:     tr.Name,
			Status:   tr.Status,
			Passed:   tr.Passed,
			Failed:   tr.Failed,
			Skipped:  tr.Skipped,
			Total:    tr.Total,
			Duration: tr.Duration,
		}
	}
	return entries
}
