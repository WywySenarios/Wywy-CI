package store

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ResultEntry represents a single test result from a results.jsonl file.
type ResultEntry struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// ParseResultsJSONL reads a results.jsonl file and returns the parsed entries.
func ParseResultsJSONL(filepath string) ([]ResultEntry, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("read results file: %w", err)
	}

	var entries []ResultEntry
	for i, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		var entry ResultEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("invalid result entry on line %d: %q", i+1, line)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}
