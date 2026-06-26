package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseResultsJSONL(t *testing.T) {
	// Create a temp directory for the results file.
	dir := t.TempDir()
	filePath := filepath.Join(dir, "results.jsonl")

	// Write 3 entries: passed, failed, passed.
	content := `{"name": "TestFoo", "status": "passed"}
{"name": "TestBar", "status": "failed"}
{"name": "TestBaz", "status": "passed"}
`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	entries, err := ParseResultsJSONL(filePath)
	if err != nil {
		t.Fatalf("ParseResultsJSONL: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// First entry.
	if entries[0].Name != "TestFoo" {
		t.Errorf("entries[0].Name: want %q, got %q", "TestFoo", entries[0].Name)
	}
	if entries[0].Status != "passed" {
		t.Errorf("entries[0].Status: want %q, got %q", "passed", entries[0].Status)
	}

	// Second entry — should be failed.
	if entries[1].Name != "TestBar" {
		t.Errorf("entries[1].Name: want %q, got %q", "TestBar", entries[1].Name)
	}
	if entries[1].Status != "failed" {
		t.Errorf("entries[1].Status: want %q, got %q", "failed", entries[1].Status)
	}

	// Third entry.
	if entries[2].Name != "TestBaz" {
		t.Errorf("entries[2].Name: want %q, got %q", "TestBaz", entries[2].Name)
	}
	if entries[2].Status != "passed" {
		t.Errorf("entries[2].Status: want %q, got %q", "passed", entries[2].Status)
	}
}

func TestParseResultsJSONLErrors(t *testing.T) {
	dir := t.TempDir()

	// 1) Empty file → empty slice, no error.
	emptyPath := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(emptyPath, []byte{}, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	entries, err := ParseResultsJSONL(emptyPath)
	if err != nil {
		t.Errorf("empty file should not error, got %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("empty file: want 0 entries, got %d", len(entries))
	}

	// 2) File with only blank lines → empty slice, no error.
	blanksPath := filepath.Join(dir, "blanks.jsonl")
	if err := os.WriteFile(blanksPath, []byte("\n\n\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	entries, err = ParseResultsJSONL(blanksPath)
	if err != nil {
		t.Errorf("blank lines should not error, got %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("blank lines: want 0 entries, got %d", len(entries))
	}

	// 3) Invalid JSON on one line → return error with the bad text.
	mixedPath := filepath.Join(dir, "mixed.jsonl")
	mixedContent := `{"name": "TestFoo", "status": "passed"}
not-json
{"name": "TestBar", "status": "failed"}
`
	if err := os.WriteFile(mixedPath, []byte(mixedContent), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, err = ParseResultsJSONL(mixedPath)
	if err == nil {
		t.Error("mixed file with invalid JSON should return error, got nil")
	} else if !strings.Contains(err.Error(), "not-json") {
		t.Errorf("error should contain the bad line text %q; got: %v", "not-json", err)
	}

	// 4) Missing file → return error.
	_, err = ParseResultsJSONL(filepath.Join(dir, "nonexistent.jsonl"))
	if err == nil {
		t.Error("missing file should return error, got nil")
	}
}

// TestParseResultsJSONLRejectsMalformedLines verifies that ParseResultsJSONL
// does NOT silently skip malformed JSON — it must return an error containing
// the defective line text so the CI runner can fail the run and surface the
// problem to the user instead of producing a false-positive "passed" status.
func TestParseResultsJSONLRejectsMalformedLines(t *testing.T) {
	dir := t.TempDir()

	// A file with a valid line followed by garbage followed by another valid line.
	badLine := "this is not json at all"
	path := filepath.Join(dir, "bad.jsonl")
	content := `{"name": "good", "status": "passed"}
` + badLine + `
{"name": "also-good", "status": "passed"}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := ParseResultsJSONL(path)
	if err == nil {
		t.Fatal("expected error for malformed JSONL line, got nil")
	}

	// The error message must contain the exact defective text.
	if !strings.Contains(err.Error(), badLine) {
		t.Errorf("error should contain the bad line text %q; got: %v", badLine, err)
	}
}

// TestParseResultsJSONLEnrichedDirect verifies that ParseResultsJSONL directly
// fills struct fields (Passed, Failed, Skipped, Total, Duration) when parsing
// enriched results.jsonl that contains count fields.
func TestParseResultsJSONLEnrichedDirect(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "results.jsonl")

	content := `{"name": "TestFoo", "status": "passed", "passed": 5, "failed": 2, "skipped": 1, "total": 8, "duration": 1.23}
`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	entries, err := ParseResultsJSONL(filePath)
	if err != nil {
		t.Fatalf("ParseResultsJSONL: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	if entries[0].Passed != 5 {
		t.Errorf("Passed: want 5, got %d", entries[0].Passed)
	}
	if entries[0].Failed != 2 {
		t.Errorf("Failed: want 2, got %d", entries[0].Failed)
	}
	if entries[0].Skipped != 1 {
		t.Errorf("Skipped: want 1, got %d", entries[0].Skipped)
	}
	if entries[0].Total != 8 {
		t.Errorf("Total: want 8, got %d", entries[0].Total)
	}
	if entries[0].Duration != 1.23 {
		t.Errorf("Duration: want 1.23, got %f", entries[0].Duration)
	}
}

// TestParseResultsJSONLLegacyWithoutCounts verifies that ParseResultsJSONL
// handles legacy JSONL without count fields — missing fields must default to
// zero rather than causing a parse error.
func TestParseResultsJSONLLegacyWithoutCounts(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "results.jsonl")

	// Old-format JSONL without count fields.
	content := `{"name": "TestFoo", "status": "passed"}
{"name": "TestBar", "status": "failed"}
`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	entries, err := ParseResultsJSONL(filePath)
	if err != nil {
		t.Fatalf("ParseResultsJSONL: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// All count fields should default to zero.
	for i, entry := range entries {
		if entry.Passed != 0 {
			t.Errorf("entries[%d].Passed: want 0, got %d", i, entry.Passed)
		}
		if entry.Failed != 0 {
			t.Errorf("entries[%d].Failed: want 0, got %d", i, entry.Failed)
		}
		if entry.Skipped != 0 {
			t.Errorf("entries[%d].Skipped: want 0, got %d", i, entry.Skipped)
		}
		if entry.Total != 0 {
			t.Errorf("entries[%d].Total: want 0, got %d", i, entry.Total)
		}
		if entry.Duration != 0.0 {
			t.Errorf("entries[%d].Duration: want 0.0, got %f", i, entry.Duration)
		}
	}
}

// TestParseResultsJSONLWithCountFields verifies that ResultEntry parses
// count fields (passed, failed, skipped, total, duration) from an enriched
// results.jsonl file. This test will fail until the ResultEntry struct gains
// these fields.
func TestParseResultsJSONLWithCountFields(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "results.jsonl")

	// Write an enriched JSONL line with all count fields.
	content := `{"name": "TestFoo", "status": "passed", "passed": 5, "failed": 2, "skipped": 1, "total": 8, "duration": 1.23}
`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	entries, err := ParseResultsJSONL(filePath)
	if err != nil {
		t.Fatalf("ParseResultsJSONL: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	// Marshal back to JSON and verify count fields survived the round-trip.
	data, err := json.Marshal(entries[0])
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	got := string(data)

	// Each count field should be present in the marshaled output.
	for _, want := range []string{
		`"passed":5`,
		`"failed":2`,
		`"skipped":1`,
		`"total":8`,
		`"duration":1.23`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("marshaled JSON should contain %s; got: %s", want, got)
		}
	}
}
