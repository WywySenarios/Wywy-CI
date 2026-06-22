package store

import (
	"os"
	"path/filepath"
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

	// 3) Invalid JSON on one line → skip that line, continue parsing others.
	mixedPath := filepath.Join(dir, "mixed.jsonl")
	mixedContent := `{"name": "TestFoo", "status": "passed"}
not-json
{"name": "TestBar", "status": "failed"}
`
	if err := os.WriteFile(mixedPath, []byte(mixedContent), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	entries, err = ParseResultsJSONL(mixedPath)
	if err != nil {
		t.Fatalf("mixed file should not error when skipping bad line, got %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("mixed file: want 2 entries, got %d", len(entries))
	}
	if entries[0].Name != "TestFoo" || entries[0].Status != "passed" {
		t.Errorf("entries[0]: want TestFoo/passed, got %s/%s", entries[0].Name, entries[0].Status)
	}
	if entries[1].Name != "TestBar" || entries[1].Status != "failed" {
		t.Errorf("entries[1]: want TestBar/failed, got %s/%s", entries[1].Name, entries[1].Status)
	}

	// 4) Missing file → return error.
	_, err = ParseResultsJSONL(filepath.Join(dir, "nonexistent.jsonl"))
	if err == nil {
		t.Error("missing file should return error, got nil")
	}
}
