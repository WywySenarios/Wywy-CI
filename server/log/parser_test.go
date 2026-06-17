package logpkg

import (
	"strings"
	"testing"
)

func TestParseValidJSON(t *testing.T) {
	line := `{"ts":"2026-06-13T10:00:00Z","service":"agentic","level":"ERROR","msg":"test failed"}`
	entry, err := ParseLine(line, "r1", "agentic", 1)
	if err != nil {
		t.Fatalf("ParseLine: %v", err)
	}

	if entry.Timestamp != "2026-06-13T10:00:00Z" {
		t.Errorf("Timestamp: want %q, got %q", "2026-06-13T10:00:00Z", entry.Timestamp)
	}
	if entry.ServiceName != "agentic" {
		t.Errorf("ServiceName: want %q, got %q", "agentic", entry.ServiceName)
	}
	if entry.Level != "ERROR" {
		t.Errorf("Level: want %q, got %q", "ERROR", entry.Level)
	}
	if entry.Content != "test failed" {
		t.Errorf("Content: want %q, got %q", "test failed", entry.Content)
	}
}

func TestParseMissingFields(t *testing.T) {
	// Plain text — not JSON.
	entry, err := ParseLine("plain text without json", "r1", "agentic", 1)
	if err != nil {
		t.Fatalf("ParseLine plain text: %v", err)
	}
	if entry.Level != "RAW" {
		t.Errorf("Level: want %q, got %q", "RAW", entry.Level)
	}
	if entry.Content != "plain text without json" {
		t.Errorf("Content: want %q, got %q", "plain text without json", entry.Content)
	}

	// JSON with missing fields — defaults applied.
	entry, err = ParseLine(`{"msg":"no level field"}`, "r1", "agentic", 2)
	if err != nil {
		t.Fatalf("ParseLine missing fields: %v", err)
	}
	if entry.Level != "INFO" {
		t.Errorf("Level: want %q, got %q", "INFO", entry.Level)
	}
	if entry.Content != "no level field" {
		t.Errorf("Content: want %q, got %q", "no level field", entry.Content)
	}

	// Empty line — should be skipped.
	_, err = ParseLine("", "r1", "agentic", 3)
	if err != ErrSkip {
		t.Fatalf("want ErrSkip for empty line, got %v", err)
	}
}

func TestParseMultiline(t *testing.T) {
	input := "{\"ts\":\"2026-06-13T10:00:00Z\",\"level\":\"ERROR\",\"msg\":\"test failed\"}\nTraceback (most recent call last):\n  File \"test.py\", line 5\nValueError: bad value\n"
	reader := strings.NewReader(input)

	entries, err := ParseFile(reader, "r1", "agentic")
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}

	// First line is JSON.
	if entries[0].Level != "ERROR" {
		t.Errorf("entry 0 Level: want ERROR, got %q", entries[0].Level)
	}
	if entries[0].Content != "test failed" {
		t.Errorf("entry 0 Content: want %q, got %q", "test failed", entries[0].Content)
	}

	// Traceback lines are RAW, same service.
	for i := 1; i < 4; i++ {
		if entries[i].Level != "RAW" {
			t.Errorf("entry %d Level: want RAW, got %q", i, entries[i].Level)
		}
		if entries[i].ServiceName != "agentic" {
			t.Errorf("entry %d ServiceName: want agentic, got %q", i, entries[i].ServiceName)
		}
		if entries[i].Content == "" {
			t.Errorf("entry %d Content should not be empty", i)
		}
	}
}

func TestStreamParse(t *testing.T) {
	input := strings.Join([]string{
		`{"ts":"T1","level":"INFO","msg":"one"}`,
		`{"ts":"T2","level":"INFO","msg":"two"}`,
		`{"ts":"T3","level":"INFO","msg":"three"}`,
		`{"ts":"T4","level":"INFO","msg":"four"}`,
		`{"ts":"T5","level":"INFO","msg":"five"}`,
	}, "\n")
	reader := strings.NewReader(input)

	ch := StreamParse(reader, "r1", "agentic")
	var entries []LogEntry
	for e := range ch {
		entries = append(entries, e)
	}

	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}
	for i, e := range entries {
		if e.LineNumber != i+1 {
			t.Errorf("entry %d LineNumber: want %d, got %d", i, i+1, e.LineNumber)
		}
	}
}
