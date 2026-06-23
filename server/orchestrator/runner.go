package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

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

// LifecycleEvent represents a run lifecycle event for external broadcast.
type LifecycleEvent struct {
	Type        string // "run_started" | "run_finished"
	RunID       string
	ServiceName string
	Suite       string
	Status      string
	Timestamp   string
}

// EventBroadcaster publishes lifecycle events (nil-able, like LogBroadcaster).
type EventBroadcaster interface {
	Publish(event LifecycleEvent)
}

// Runner orchestrates test runs against the store.
type Runner struct {
	store            *store.Store
	cmd              CommandRunner
	validServices    map[string]bool        // nil = skip validation
	broadcaster      LogBroadcaster         // nil = skip broadcasting
	eventBroadcaster EventBroadcaster       // nil = skip event broadcasting
	RunsDir          string                 // base directory for run output (stdout.log, stderr.log, results.jsonl)
	resolver         *ServiceScriptResolver // script resolution; nil = skip detached execution
}

// SetBroadcaster configures a broadcaster for streaming log output.
func (r *Runner) SetBroadcaster(b LogBroadcaster) {
	r.broadcaster = b
}

// SetEventBroadcaster configures an event broadcaster for lifecycle events.
func (r *Runner) SetEventBroadcaster(eb EventBroadcaster) {
	r.eventBroadcaster = eb
}

// SetResolver configures the script resolver for detached script execution.
func (r *Runner) SetResolver(resolver *ServiceScriptResolver) {
	r.resolver = resolver
}

// ListSuites returns the available test suite names for a service.
// Returns nil if no resolver is configured (no-op without an error).
func (r *Runner) ListSuites(service string) ([]string, error) {
	if r.resolver == nil {
		return nil, nil
	}
	return r.resolver.ListSuites(service)
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

		if r.eventBroadcaster != nil {
			r.eventBroadcaster.Publish(LifecycleEvent{
				Type:        "run_started",
				RunID:       runID,
				ServiceName: svc,
				Suite:       suite,
				Status:      "running",
				Timestamp:   timestamp(),
			})
		}
	}

	go r.executeServices(ctx, runID, suite, services)
	return run, nil
}

// executeDetached resolves the script path, spawns it detached, monitors for
// results.jsonl, and returns (exitCode, status) based on the parsed results.
func (r *Runner) executeDetached(ctx context.Context, dr DetachedCommandRunner, runID, serviceName, suite string) (int, string) {
	scriptPath, err := r.resolver.ResolveScriptPath(serviceName, suite)
	if err != nil {
		return 1, "failed"
	}

	outputDir := filepath.Join(r.RunsDir, runID, serviceName)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return 1, "failed"
	}

	args := BuildScriptArgs(ScriptInvocation{
		RunID:     runID,
		OutputDir: outputDir,
	})

	if _, err := dr.StartDetached(ctx, outputDir, scriptPath, args...); err != nil {
		return 1, "failed"
	}

	results, stdout, stderr, err := MonitorScriptOutput(ctx, outputDir, 30*time.Minute)
	if err != nil {
		if ctx.Err() != nil {
			return -1, "cancelled"
		}
		return 1, "failed"
	}

	// Persist captured stdout/stderr as log entries.
	if stdout != "" || stderr != "" {
		r.storeScriptOutput(runID, serviceName, stdout, stderr)
	}

	// Any non-passed result → service failed.
	for _, res := range results {
		if res.Status != "passed" {
			return 1, "failed"
		}
	}
	return 0, "passed"
}

// storeScriptOutput splits captured stdout/stderr into log entries and persists them.
func (r *Runner) storeScriptOutput(runID, serviceName, stdout, stderr string) {
	now := timestamp()
	var entries []store.LogEntry
	lineNum := 0

	for _, line := range strings.Split(stdout, "\n") {
		lineNum++
		if line == "" {
			continue
		}
		entries = append(entries, store.LogEntry{
			RunID:       runID,
			ServiceName: serviceName,
			LineNumber:  lineNum,
			Timestamp:   now,
			Level:       "RAW",
			Content:     line,
		})
	}
	for _, line := range strings.Split(stderr, "\n") {
		lineNum++
		if line == "" {
			continue
		}
		entries = append(entries, store.LogEntry{
			RunID:       runID,
			ServiceName: serviceName,
			LineNumber:  lineNum,
			Timestamp:   now,
			Level:       "RAW",
			Content:     line,
		})
	}

	if len(entries) > 0 {
		_ = r.store.InsertLogEntries(entries)
	}
}

// DetectOrphanedRuns finds runs stuck in "running" status (e.g., after server restart)
// and marks them as "failed" when no output files exist, or updates their status based
// on existing results.jsonl files.
func (r *Runner) DetectOrphanedRuns() {
	runs, err := r.store.ListRuns()
	if err != nil {
		return
	}

	for _, run := range runs {
		if run.Status != "running" {
			continue
		}

		services, err := r.store.ListRunServices(run.ID)
		if err != nil || len(services) == 0 {
			// No services to recover — just mark the run as failed.
			r.store.UpdateRunStatus(run.ID, "failed", timestamp())
			continue
		}

		allPassed := true
		for _, svc := range services {
			resultsPath := filepath.Join(r.RunsDir, run.ID, svc.ServiceName, "results.jsonl")

			if _, err := os.Stat(resultsPath); err == nil {
				entries, parseErr := store.ParseResultsJSONL(resultsPath)
				if parseErr != nil {
					r.store.UpdateRunService(run.ID, svc.ServiceName, 1, "failed", timestamp())
					allPassed = false
					continue
				}
				svcPassed := true
				for _, e := range entries {
					if e.Status != "passed" {
						svcPassed = false
						break
					}
				}
				if svcPassed {
					r.store.UpdateRunService(run.ID, svc.ServiceName, 0, "passed", timestamp())
				} else {
					r.store.UpdateRunService(run.ID, svc.ServiceName, 1, "failed", timestamp())
					allPassed = false
				}
			} else {
				// No output — orphaned before completion.
				r.store.UpdateRunService(run.ID, svc.ServiceName, 1, "failed", timestamp())
				allPassed = false
			}
		}

		if allPassed {
			r.store.UpdateRunStatus(run.ID, "passed", timestamp())
		} else {
			r.store.UpdateRunStatus(run.ID, "failed", timestamp())
		}
	}
}

func (r *Runner) executeServices(ctx context.Context, runID string, suite string, services []string) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	overallPassed := true
	cancelled := false

	for _, svc := range services {
		wg.Add(1)
		go func(serviceName string) {
			defer wg.Done()

			var code int
			var status string

			if dr, ok := r.cmd.(DetachedCommandRunner); ok && r.RunsDir != "" && r.resolver != nil {
				// Detached execution path: resolve script, spawn, monitor, parse results.
				code, status = r.executeDetached(ctx, dr, runID, serviceName, suite)
			}
			if status == "" {
				code = 1
				status = "failed"
				msg := fmt.Sprintf("Run failed: no execution path configured for service %q. "+
					"Set a resolver and RunsDir on the Runner.", serviceName)
				_ = r.store.InsertLogEntries([]store.LogEntry{{
					RunID:       runID,
					ServiceName: serviceName,
					LineNumber:  0,
					Timestamp:   timestamp(),
					Level:       "ERROR",
					Content:     msg,
				}})
			}

			endTime := timestamp()
			mu.Lock()
			switch status {
			case "cancelled":
				cancelled = true
				overallPassed = false
			case "failed":
				overallPassed = false
			}
			mu.Unlock()

			if r.eventBroadcaster != nil {
				r.eventBroadcaster.Publish(LifecycleEvent{
					Type:        "run_finished",
					RunID:       runID,
					ServiceName: serviceName,
					Suite:       suite,
					Status:      status,
					Timestamp:   endTime,
				})
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
