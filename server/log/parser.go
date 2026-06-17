package logpkg

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
)

// ErrSkip indicates that a log line should be skipped (e.g., empty lines).
var ErrSkip = errors.New("skip log line")

// LogEntry represents a parsed line of test output.
type LogEntry struct {
	Timestamp   string
	ServiceName string
	Level       string
	Content     string
	LineNumber  int
}

// rawEntry is the JSON structure parsed from a log line.
type rawEntry struct {
	TS      string `json:"ts"`
	Service string `json:"service"`
	Level   string `json:"level"`
	Msg     string `json:"msg"`
}

// ParseLine parses a single JSON-lines log entry.
func ParseLine(line string, runID, serviceName string, lineNumber int) (LogEntry, error) {
	if line == "" {
		return LogEntry{}, ErrSkip
	}

	var raw rawEntry
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		// Non-JSON line — wrap as RAW.
		return LogEntry{
			Level:      "RAW",
			Content:    line,
			LineNumber: lineNumber,
		}, nil
	}

	level := raw.Level
	if level == "" {
		level = "INFO"
	}

	return LogEntry{
		Timestamp:   raw.TS,
		ServiceName: raw.Service,
		Level:       level,
		Content:     raw.Msg,
		LineNumber:  lineNumber,
	}, nil
}

// ParseFile reads all lines from reader and returns parsed log entries.
func ParseFile(reader io.Reader, runID, serviceName string) ([]LogEntry, error) {
	var entries []LogEntry
	for e := range StreamParse(reader, runID, serviceName) {
		entries = append(entries, e)
	}
	return entries, nil
}

// StreamParse reads lines from reader and sends parsed entries on a channel.
// The channel is closed when the reader is exhausted.
func StreamParse(reader io.Reader, runID, serviceName string) <-chan LogEntry {
	ch := make(chan LogEntry)
	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(reader)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			entry, err := ParseLine(scanner.Text(), runID, serviceName, lineNum)
			if errors.Is(err, ErrSkip) {
				continue
			}
			if err != nil {
				return
			}
			if entry.ServiceName == "" {
				entry.ServiceName = serviceName
			}
			ch <- entry
		}
	}()
	return ch
}
