package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"wywy-website/ci/server/log"

	"wywy-website/ci/server/store"
)

// LogMessage represents a log event for broadcast.
type LogMessage struct {
	Type        string `json:"type"`
	RunID       string `json:"run_id"`
	ServiceName string `json:"service_name,omitempty"`
	Level       string `json:"level,omitempty"`
	Content     string `json:"content,omitempty"`
	Status      string `json:"status,omitempty"`
}

// LogBroadcaster is the interface for streaming log entries to connected clients.
// The api.Broadcaster satisfies this interface structurally.
type LogBroadcaster interface {
	Send(runID string, msg LogMessage)
	Done(runID string, status string)
}

// Runner orchestrates test runs against the store.
type Runner struct {
	store         *store.Store
	cmd           CommandRunner
	validServices map[string]bool // nil = skip validation
	broadcaster   LogBroadcaster  // nil = skip broadcasting
	LogsDir       string          // directory for raw log files; empty = skip file writing
}

// SetBroadcaster configures a broadcaster for streaming log output.
func (r *Runner) SetBroadcaster(b LogBroadcaster) {
	r.broadcaster = b
}

// NewRunner creates a new Runner.
func NewRunner(s *store.Store, cmd CommandRunner) *Runner {
	return &Runner{store: s, cmd: cmd}
}

// NewRunnerWithServices creates a Runner with a validated set of allowed service names.
// An unknown service passed to StartRun will return an error without creating a run.
func NewRunnerWithServices(s *store.Store, cmd CommandRunner, validServices map[string]bool) *Runner {
	return &Runner{store: s, cmd: cmd, validServices: validServices}
}

// timestamp returns the current UTC time in RFC3339 format.
func timestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// writeRawLog writes the raw command output to {LogsDir}/{runID}/{serviceName}.log.
// LogsDir empty or output empty → no-op. Errors are non-fatal.
func (r *Runner) writeRawLog(runID, serviceName, output string) {
	if r.LogsDir == "" || output == "" {
		return
	}
	runDir := filepath.Join(r.LogsDir, runID)
	if err := os.MkdirAll(runDir, 0755); err != nil {
		return
	}
	logPath := filepath.Join(runDir, serviceName+".log")
	_ = os.WriteFile(logPath, []byte(output), 0644)
}

// StartRun starts a test run for the given services and suite.
func (r *Runner) StartRun(ctx context.Context, services []string, suite string) (*store.Run, error) {
	if r.validServices != nil {
		for _, svc := range services {
			if !r.validServices[svc] {
				return nil, fmt.Errorf("unknown service: %s", svc)
			}
		}
	}

	runID := fmt.Sprintf("run-%d", time.Now().UnixNano())

	run := &store.Run{
		ID:        runID,
		CreatedAt: timestamp(),
		Status:    "running",
	}
	if err := r.store.CreateRun(run); err != nil {
		return nil, fmt.Errorf("create run: %w", err)
	}

	for _, svc := range services {
		rs := &store.RunService{
			RunID:       runID,
			ServiceName: svc,
			Suite:       suite,
			Status:      "pending",
		}
		if err := r.store.CreateRunService(rs); err != nil {
			return nil, fmt.Errorf("create run_service %s: %w", svc, err)
		}
	}

	go r.executeServices(ctx, runID, services)
	return run, nil
}

func (r *Runner) executeServices(ctx context.Context, runID string, services []string) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	overallPassed := true
	cancelled := false

	for _, svc := range services {
		wg.Add(1)
		go func(serviceName string) {
			defer wg.Done()

			code, output, err := r.cmd.Run(ctx, "sh", "-c", fmt.Sprintf("echo 'test output for %s'", serviceName))
			endTime := timestamp()

			r.writeRawLog(runID, serviceName, output)

			status := "passed"
			if ctx.Err() != nil {
				status = "cancelled"
				mu.Lock()
				cancelled = true
				overallPassed = false
				mu.Unlock()
			} else if err != nil || code != 0 {
				status = "failed"
				mu.Lock()
				overallPassed = false
				mu.Unlock()
			}

			// Parse and persist log output.
			if output != "" {
				if parsed, parseErr := logpkg.ParseFile(strings.NewReader(output), runID, serviceName); parseErr == nil && len(parsed) > 0 {
					entries := make([]store.LogEntry, len(parsed))
					for i, e := range parsed {
						entries[i] = store.LogEntry{
							RunID:       runID,
							ServiceName: e.ServiceName,
							LineNumber:  e.LineNumber,
							Timestamp:   e.Timestamp,
							Level:       e.Level,
							Content:     e.Content,
						}
						// Broadcast each parsed entry to WebSocket clients.
						if r.broadcaster != nil {
							r.broadcaster.Send(runID, LogMessage{
								Type:        "log",
								RunID:       runID,
								ServiceName: e.ServiceName,
								Level:       e.Level,
								Content:     e.Content,
							})
						}
					}
					if storeErr := r.store.InsertLogEntries(entries); storeErr != nil {
						// Non-fatal: run result is already determined, log capture failure
						// should not change the run outcome.
					}
				}
			}

			r.store.UpdateRunService(runID, serviceName, code, status, endTime)
		}(svc)
	}
	wg.Wait()

	var finalStatus string
	switch {
	case cancelled:
		finalStatus = "cancelled"
	case !overallPassed:
		finalStatus = "failed"
	default:
		finalStatus = "passed"
	}
	r.store.UpdateRunStatus(runID, finalStatus, timestamp())

	if r.broadcaster != nil {
		r.broadcaster.Done(runID, finalStatus)
	}
}
