package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"wywy-website/ci/server/orchestrator"
	"wywy-website/ci/server/store"

	"github.com/coder/websocket"
)

// fakeRunner returns a fixed exit code and output for API tests.
type fakeRunner struct {
	exitCode int
	output   string
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) (int, string, error) {
	return f.exitCode, f.output, nil
}

func (f *fakeRunner) StartDetached(ctx context.Context, outputDir, name string, args ...string) (*exec.Cmd, error) {
	os.MkdirAll(outputDir, 0755)
	os.WriteFile(filepath.Join(outputDir, "stdout.log"), []byte("script ran\n"), 0644)
	os.WriteFile(filepath.Join(outputDir, "stderr.log"), []byte(""), 0644)
	os.WriteFile(filepath.Join(outputDir, "results.jsonl"), []byte(`{"name":"t1","status":"passed"}`+"\n"), 0644)

	cmd := exec.CommandContext(ctx, "sh", "-c", "exit 0")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Start()
	return cmd, nil
}

func TestListRunsEmpty(t *testing.T) {
	s := newTestStore(t)
	srv := newTestServer(t, s, nil, nil)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/runs")
	if err != nil {
		t.Fatalf("GET /api/runs: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: want 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: want application/json, got %q", ct)
	}

	var body []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body) != 0 {
		t.Errorf("body: want empty array, got %d elements", len(body))
	}
}

func TestCreateRun(t *testing.T) {
	s := newTestStore(t)
	runner := orchestrator.NewRunner(s, &fakeRunner{exitCode: 0, output: "ok"})
	srv := newTestServer(t, s, runner, nil)
	defer srv.Close()

	body := bytes.NewBufferString(`{"services":["agentic"],"suite":"test"}`)
	resp, err := http.Post(srv.URL+"/api/runs", "application/json", body)
	if err != nil {
		t.Fatalf("POST /api/runs: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("status: want 202, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: want application/json, got %q", ct)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if id, ok := result["id"]; !ok || id == "" {
		t.Error("response missing 'id'")
	}
	if status, ok := result["status"]; !ok || status != "running" {
		t.Errorf("status: want running, got %v", status)
	}
	if ca, ok := result["created_at"]; !ok || ca == "" {
		t.Error("response missing 'created_at'")
	}
}

func TestCreateRunInvalidService(t *testing.T) {
	s := newTestStore(t)
	runner := orchestrator.NewRunner(s, &fakeRunner{exitCode: 0, output: "ok"})
	srv := newTestServer(t, s, runner, map[string]bool{"agentic": true})
	defer srv.Close()

	body := bytes.NewBufferString(`{"services":["nonexistent"],"suite":"test"}`)
	resp, err := http.Post(srv.URL+"/api/runs", "application/json", body)
	if err != nil {
		t.Fatalf("POST /api/runs: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if _, ok := result["error"]; !ok {
		t.Error("response missing 'error' field")
	}
}

func TestGetRunFound(t *testing.T) {
	s := newTestStore(t)

	run := &store.Run{
		ID: "r1", CreatedAt: "2026-01-01T00:00:00Z", Status: "running",
	}
	if err := s.CreateRun(run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	srv := newTestServer(t, s, nil, nil)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/runs/r1")
	if err != nil {
		t.Fatalf("GET /api/runs/r1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: want 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: want application/json, got %q", ct)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if id, ok := result["id"]; !ok || id != "r1" {
		t.Errorf("id: want r1, got %v", id)
	}
	if status, ok := result["status"]; !ok || status != "running" {
		t.Errorf("status: want running, got %v", status)
	}
	if ca, ok := result["created_at"]; !ok || ca == "" {
		t.Error("missing 'created_at'")
	}
}

func TestGetRunReturnsServicesWithExitCode(t *testing.T) {
	s := newTestStore(t)

	run := &store.Run{
		ID: "r-exit", CreatedAt: "2026-01-01T00:00:00Z", Status: "failed",
	}
	if err := s.CreateRun(run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	exitCode := 42
	rs := &store.RunService{
		RunID: "r-exit", ServiceName: "ci", Suite: "test",
		Status: "failed", ExitCode: &exitCode,
		EndTime: "2026-01-01T00:01:00Z",
	}
	if err := s.CreateRunService(rs); err != nil {
		t.Fatalf("CreateRunService: %v", err)
	}

	srv := newTestServer(t, s, nil, nil)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/runs/r-exit")
	if err != nil {
		t.Fatalf("GET /api/runs/r-exit: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: want 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: want application/json, got %q", ct)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	// The response MUST include the services array with exit_code.
	services, ok := result["services"].([]any)
	if !ok {
		t.Fatal("response missing 'services' array — exit codes are not exposed to the frontend")
	}
	if len(services) == 0 {
		t.Fatal("expected at least one service in 'services' array")
	}

	svc := services[0].(map[string]any)

	if name, ok := svc["service_name"]; !ok || name != "ci" {
		t.Errorf("service_name: want ci, got %v", name)
	}
	gotExitCode, ok := svc["exit_code"].(float64)
	if !ok || int(gotExitCode) != 42 {
		t.Errorf("exit_code: want 42, got %v", svc["exit_code"])
	}
}

func TestCreateRunWithSingleServiceReturnsOneServiceOnGet(t *testing.T) {
	s := newTestStore(t)
	runner := orchestrator.NewRunner(s, &fakeRunner{exitCode: 0, output: "ok"})

	// Note: validServices=nil means no validation — any service name is accepted.
	srv := newTestServer(t, s, runner, nil)
	defer srv.Close()

	// Create a run with ONE service.
	body := bytes.NewBufferString(`{"services":["agentic"],"suite":"test"}`)
	resp, err := http.Post(srv.URL+"/api/runs", "application/json", body)
	if err != nil {
		t.Fatalf("POST /api/runs: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("POST status: want 202, got %d", resp.StatusCode)
	}

	// Extract the run ID from the POST response.
	var createResult map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&createResult); err != nil {
		t.Fatalf("decode POST body: %v", err)
	}
	runID, ok := createResult["id"].(string)
	if !ok || runID == "" {
		t.Fatal("POST response missing 'id'")
	}

	// Give the goroutine a moment to complete so services get exit codes.
	time.Sleep(200 * time.Millisecond)

	// GET the run detail.
	getResp, err := http.Get(srv.URL + "/api/runs/" + runID)
	if err != nil {
		t.Fatalf("GET /api/runs/%s: %v", runID, err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("GET status: want 200, got %d", getResp.StatusCode)
	}

	var getResult map[string]any
	if err := json.NewDecoder(getResp.Body).Decode(&getResult); err != nil {
		t.Fatalf("decode GET body: %v", err)
	}

	services, ok := getResult["services"].([]any)
	if !ok {
		t.Fatal("GET response is missing 'services' array — exit codes are not exposed to the frontend")
	}

	// MUST have exactly one service — only the tested service.
	if len(services) != 1 {
		t.Fatalf("services: want exactly 1 (the tested service), got %d — "+
			"bug: exit codes for untested services are leaked to the UI", len(services))
	}

	svc := services[0].(map[string]any)
	if name, ok := svc["service_name"]; !ok || name != "agentic" {
		t.Errorf("service_name: want 'agentic', got %v", name)
	}
	// Exit code must be non-nil — the service was executed (even if failed in test setup).
	if exitCode, ok := svc["exit_code"]; !ok || exitCode == nil {
		t.Error("service entry has nil exit_code — service was not processed")
	}
}

func TestGetRunNotFound(t *testing.T) {
	s := newTestStore(t)

	srv := newTestServer(t, s, nil, nil)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/runs/nonexistent")
	if err != nil {
		t.Fatalf("GET /api/runs/nonexistent: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: want 404, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: want application/json, got %q", ct)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if _, ok := result["error"]; !ok {
		t.Error("response missing 'error' field")
	}
}

func TestListRunsWithData(t *testing.T) {
	s := newTestStore(t)

	run1 := &store.Run{
		ID: "run-one", CreatedAt: "2026-01-01T00:00:00Z", Status: "passed",
	}
	run2 := &store.Run{
		ID: "run-two", CreatedAt: "2026-06-01T00:00:00Z", Status: "failed",
	}
	if err := s.CreateRun(run1); err != nil {
		t.Fatalf("CreateRun(run1): %v", err)
	}
	if err := s.CreateRun(run2); err != nil {
		t.Fatalf("CreateRun(run2): %v", err)
	}

	srv := newTestServer(t, s, nil, nil)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/runs")
	if err != nil {
		t.Fatalf("GET /api/runs: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: want 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: want application/json, got %q", ct)
	}

	var result []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("want 2 runs, got %d", len(result))
	}

	// Newest first (run2 has later date).
	if id, ok := result[0]["id"]; !ok || id != "run-two" {
		t.Errorf("result[0].id: want run-two, got %v", id)
	}
	if status, ok := result[0]["status"]; !ok || status != "failed" {
		t.Errorf("result[0].status: want failed, got %v", status)
	}

	if id, ok := result[1]["id"]; !ok || id != "run-one" {
		t.Errorf("result[1].id: want run-one, got %v", id)
	}
	if status, ok := result[1]["status"]; !ok || status != "passed" {
		t.Errorf("result[1].status: want passed, got %v", status)
	}
}

func TestGetRunLogsFiltered(t *testing.T) {
	s := newTestStore(t)

	run := &store.Run{
		ID: "r1", CreatedAt: "2026-01-01T00:00:00Z", Status: "running",
	}
	if err := s.CreateRun(run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	var entries []store.LogEntry
	for i := 0; i < 10; i++ {
		entries = append(entries, store.LogEntry{
			RunID: "r1", ServiceName: "agentic",
			LineNumber: i, Timestamp: "2026-01-01T00:00:00Z",
			Level: "INFO", Content: "heartbeat",
		})
	}
	for i := 0; i < 5; i++ {
		entries = append(entries, store.LogEntry{
			RunID: "r1", ServiceName: "agentic",
			LineNumber: 10 + i, Timestamp: "2026-01-01T00:00:00Z",
			Level: "ERROR", Content: "timeout connecting to cache",
		})
	}
	for i := 0; i < 5; i++ {
		entries = append(entries, store.LogEntry{
			RunID: "r1", ServiceName: "agentic",
			LineNumber: 20 + i, Timestamp: "2026-01-01T00:00:00Z",
			Level: "WARN", Content: "retry attempt",
		})
	}
	if err := s.InsertLogEntries(entries); err != nil {
		t.Fatalf("InsertLogEntries: %v", err)
	}

	srv := newTestServer(t, s, nil, nil)
	defer srv.Close()

	// Test 1: filter by level.
	resp, err := http.Get(srv.URL + "/api/runs/r1/logs/agentic?level=ERROR")
	if err != nil {
		t.Fatalf("GET logs by level: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: want 200, got %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: want application/json, got %q", ct)
	}
	var levelResult []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&levelResult); err != nil {
		t.Fatalf("decode level filter: %v", err)
	}
	resp.Body.Close()

	if len(levelResult) != 5 {
		t.Errorf("level filter: want 5 ERROR entries, got %d", len(levelResult))
	}
	for _, entry := range levelResult {
		if entry["level"] != "ERROR" {
			t.Errorf("level filter: entry has level %v, want ERROR", entry["level"])
		}
	}

	// Test 2: filter by search with limit.
	resp, err = http.Get(srv.URL + "/api/runs/r1/logs/agentic?search=timeout&limit=5")
	if err != nil {
		t.Fatalf("GET logs by search: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: want 200, got %d", resp.StatusCode)
	}
	var searchResult []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&searchResult); err != nil {
		t.Fatalf("decode search filter: %v", err)
	}
	resp.Body.Close()

	if len(searchResult) > 5 {
		t.Errorf("search+limit: want ≤5 entries, got %d", len(searchResult))
	}
	if len(searchResult) == 0 {
		t.Error("search+limit: expected at least 1 entry containing 'timeout'")
	}
}

func TestGetRunLogsAllServices(t *testing.T) {
	s := newTestStore(t)

	run := &store.Run{
		ID: "r1", CreatedAt: "2026-01-01T00:00:00Z", Status: "passed",
	}
	if err := s.CreateRun(run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	entries := []store.LogEntry{
		{RunID: "r1", ServiceName: "agentic", LineNumber: 0, Content: "log from agentic"},
		{RunID: "r1", ServiceName: "ci", LineNumber: 1, Content: "log from ci"},
	}
	if err := s.InsertLogEntries(entries); err != nil {
		t.Fatalf("InsertLogEntries: %v", err)
	}

	srv := newTestServer(t, s, nil, nil)
	defer srv.Close()

	// No service name — endpoint should return logs from all services.
	resp, err := http.Get(srv.URL + "/api/runs/r1/logs")
	if err != nil {
		t.Fatalf("GET /api/runs/r1/logs: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: want 200, got %d", resp.StatusCode)
	}

	var result []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("want 2 log entries (both services), got %d", len(result))
	}

	// Both services must be represented.
	services := make(map[string]bool)
	for _, entry := range result {
		services[entry["service_name"].(string)] = true
	}
	if !services["agentic"] {
		t.Error("missing log from service 'agentic'")
	}
	if !services["ci"] {
		t.Error("missing log from service 'ci'")
	}
}

func TestListActiveRuns(t *testing.T) {
	s := newTestStore(t)

	// Run 1: running with service "agentic".
	run1 := &store.Run{
		ID: "run-active", CreatedAt: "2026-06-01T00:00:00Z", Status: "running",
	}
	if err := s.CreateRun(run1); err != nil {
		t.Fatalf("CreateRun(run1): %v", err)
	}
	if err := s.CreateRunService(&store.RunService{
		RunID: "run-active", ServiceName: "agentic", Suite: "test", Status: "running",
	}); err != nil {
		t.Fatalf("CreateRunService(agentic): %v", err)
	}

	// Run 2: completed, service "ci" is not active.
	run2 := &store.Run{
		ID: "run-done", CreatedAt: "2026-06-01T00:01:00Z", Status: "passed",
	}
	if err := s.CreateRun(run2); err != nil {
		t.Fatalf("CreateRun(run2): %v", err)
	}
	if err := s.CreateRunService(&store.RunService{
		RunID: "run-done", ServiceName: "ci", Suite: "test", Status: "passed",
	}); err != nil {
		t.Fatalf("CreateRunService(ci): %v", err)
	}

	srv := newTestServer(t, s, nil, nil)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/runs/active")
	if err != nil {
		t.Fatalf("GET /api/runs/active: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: want 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: want application/json, got %q", ct)
	}

	var outer map[string]map[string]map[string]bool
	if err := json.NewDecoder(resp.Body).Decode(&outer); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	suites, ok := outer["active_suites"]
	if !ok {
		t.Fatal("response missing 'active_suites'")
	}

	if !suites["agentic"]["test"] {
		t.Error("agentic/test should be active (running)")
	}
	if suites["ci"] != nil && suites["ci"]["test"] {
		t.Error("ci should NOT be active (completed)")
	}
}

func TestCreateRunEmitsRunStartedEvent(t *testing.T) {
	srv, conn, ctx, cancel := setupEventTest(t)
	defer cancel()
	defer srv.Close()
	defer conn.Close(websocket.StatusNormalClosure, "")

	// POST to create a run.
	body := bytes.NewBufferString(`{"services":["agentic"],"suite":"test"}`)
	resp, err := http.Post(srv.URL+"/api/runs", "application/json", body)
	if err != nil {
		t.Fatalf("POST /api/runs: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("status: want 202, got %d", resp.StatusCode)
	}

	// Read the run_started event from the WebSocket.
	_, msg, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("WebSocket read: %v", err)
	}

	var received RunEvent
	if err := json.Unmarshal(msg, &received); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if received.Type != "run_started" {
		t.Errorf("Type: want %q, got %q", "run_started", received.Type)
	}
	if received.ServiceName != "agentic" {
		t.Errorf("ServiceName: want %q, got %q", "agentic", received.ServiceName)
	}
	if received.Status != "running" {
		t.Errorf("Status: want %q, got %q", "running", received.Status)
	}
}

// TestCreateRunEmitsFullLifecycleEvents verifies that both run_started and
// run_finished events are emitted via WebSocket when a run is created.
func TestCreateRunEmitsFullLifecycleEvents(t *testing.T) {
	srv, conn, ctx, cancel := setupEventTest(t)
	defer cancel()
	defer srv.Close()
	defer conn.Close(websocket.StatusNormalClosure, "")

	// POST to create a run.
	body := bytes.NewBufferString(`{"services":["agentic"],"suite":"test"}`)
	resp, err := http.Post(srv.URL+"/api/runs", "application/json", body)
	if err != nil {
		t.Fatalf("POST /api/runs: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("status: want 202, got %d", resp.StatusCode)
	}

	// Read the run_started event from the WebSocket.
	_, msg, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("WebSocket read run_started: %v", err)
	}

	var started RunEvent
	if err := json.Unmarshal(msg, &started); err != nil {
		t.Fatalf("unmarshal run_started: %v", err)
	}

	if started.Type != "run_started" {
		t.Errorf("started Type: want %q, got %q", "run_started", started.Type)
	}
	if started.ServiceName != "agentic" {
		t.Errorf("started ServiceName: want %q, got %q", "agentic", started.ServiceName)
	}
	if started.Status != "running" {
		t.Errorf("started Status: want %q, got %q", "running", started.Status)
	}
	if started.RunID == "" {
		t.Error("started RunID: expected non-empty")
	}

	// Read the run_finished event from the WebSocket.
	_, msg, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("WebSocket read run_finished: %v", err)
	}

	var finished RunEvent
	if err := json.Unmarshal(msg, &finished); err != nil {
		t.Fatalf("unmarshal run_finished: %v", err)
	}

	if finished.Type != "run_finished" {
		t.Errorf("finished Type: want %q, got %q", "run_finished", finished.Type)
	}
	if finished.ServiceName != "agentic" {
		t.Errorf("finished ServiceName: want %q, got %q", "agentic", finished.ServiceName)
	}
	if finished.Status != "passed" {
		t.Errorf("finished Status: want %q, got %q", "passed", finished.Status)
	}
	if finished.RunID == "" {
		t.Error("finished RunID: expected non-empty")
	}
	if finished.RunID != started.RunID {
		t.Error("finished RunID does not match started RunID")
	}
}

// TestGetRunReturnsAggregatedCounts verifies that GET /api/runs/{id} includes
// aggregated count fields (passed, failed, skipped) at the run level.
// Counts are summed from the run's services and appear alongside the services array.
func TestGetRunReturnsAggregatedCounts(t *testing.T) {
	s := newTestStore(t)
	createRunWithTwoServices(t, s, "r-aggr")
	srv := newTestServer(t, s, nil, nil)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/runs/r-aggr")
	if err != nil {
		t.Fatalf("GET /api/runs/r-aggr: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: want 200, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	// Run-level aggregated counts should be passed=8, failed=3, skipped=1.
	assertAggregatedCounts(t, result, 8, 3, 1)

	// Per-service counts should still be present in the services array.
	services, ok := result["services"].([]any)
	if !ok {
		t.Fatal("response missing 'services' array")
	}
	if len(services) != 2 {
		t.Fatalf("services: want 2, got %d", len(services))
	}
}

// TestListRunsReturnsAggregatedCounts verifies that GET /api/runs includes
// aggregated count fields (passed, failed, skipped) on each run object.
func TestListRunsReturnsAggregatedCounts(t *testing.T) {
	s := newTestStore(t)
	createRunWithTwoServices(t, s, "r-list")
	srv := newTestServer(t, s, nil, nil)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/runs")
	if err != nil {
		t.Fatalf("GET /api/runs: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: want 200, got %d", resp.StatusCode)
	}

	var result []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("want 1 run, got %d", len(result))
	}

	// Run-level aggregated counts should be passed=8, failed=3, skipped=1.
	assertAggregatedCounts(t, result[0], 8, 3, 1)
}

// newTestServer creates an httptest.Server with the given store and runner.
func newTestServer(t *testing.T, st *store.Store, runner *orchestrator.Runner, validServices map[string]bool) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	h := &Handler{Store: st, Runner: runner, ValidServices: validServices}
	h.RegisterRoutes(mux)
	return httptest.NewServer(mux)
}

// createRunWithTwoServices creates a run with two services whose counts sum
// to passed=8, failed=3, skipped=1.
func createRunWithTwoServices(t *testing.T, s *store.Store, runID string) {
	t.Helper()
	run := &store.Run{ID: runID, CreatedAt: "2026-06-01T00:00:00Z", Status: "passed"}
	if err := s.CreateRun(run); err != nil {
		t.Fatalf("CreateRun(%q): %v", runID, err)
	}
	if err := s.CreateRunService(&store.RunService{
		RunID: runID, ServiceName: "agentic", Suite: "test",
		Status: "passed", Passed: 5, Failed: 2, Skipped: 1,
	}); err != nil {
		t.Fatalf("CreateRunService(%q/agentic): %v", runID, err)
	}
	if err := s.CreateRunService(&store.RunService{
		RunID: runID, ServiceName: "ci", Suite: "test",
		Status: "failed", Passed: 3, Failed: 1, Skipped: 0,
	}); err != nil {
		t.Fatalf("CreateRunService(%q/ci): %v", runID, err)
	}
}

// assertAggregatedCounts checks that data contains the expected passed/failed/skipped
// keys and values, as decoded from JSON (float64 for numeric fields).
func assertAggregatedCounts(t *testing.T, data map[string]any, wantPassed, wantFailed, wantSkipped int) {
	t.Helper()
	for _, key := range []string{"passed", "failed", "skipped"} {
		if _, ok := data[key]; !ok {
			t.Errorf("response missing aggregated %q", key)
		}
	}
	gotPassed, _ := data["passed"].(float64)
	gotFailed, _ := data["failed"].(float64)
	gotSkipped, _ := data["skipped"].(float64)
	if gotPassed != float64(wantPassed) {
		t.Errorf("passed: want %d, got %v", wantPassed, gotPassed)
	}
	if gotFailed != float64(wantFailed) {
		t.Errorf("failed: want %d, got %v", wantFailed, gotFailed)
	}
	if gotSkipped != float64(wantSkipped) {
		t.Errorf("skipped: want %d, got %v", wantSkipped, gotSkipped)
	}
}

// buildServiceRepoMap reads services.txt (name,repo format) and returns a
// map of service name → repo name, borrowing the same source as main.go.
// Returns nil if the file cannot be read.
func buildServiceRepoMap(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	repos := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ",", 2)
		if len(parts) > 1 {
			repos[parts[0]] = parts[1]
		}
	}
	return repos
}

// newTestStore opens an in-memory store for API tests.
func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// setupEventTest creates a test server with EventBroadcaster wired to both the
// Handler (for the WebSocket endpoint) and the Runner (for publishing events),
// and returns a connected WebSocket client. Caller is responsible for
// closing/cleanup of all return values.
func setupEventTest(t *testing.T) (srv *httptest.Server, conn *websocket.Conn, ctx context.Context, cancel context.CancelFunc) {
	t.Helper()
	st := newTestStore(t)
	eb := NewEventBroadcaster()

	// Borrow the production runner configuration: read services.txt
	// to build the resolver, set RunsDir.
	repos := buildServiceRepoMap("/etc/Wywy-Website-Control/services.txt")
	runner := orchestrator.NewRunner(st, &fakeRunner{exitCode: 0, output: "ok"})
	runner.SetEventBroadcaster(&eventBroadcasterAdapter{inner: eb})
	runner.RunsDir = t.TempDir()
	if repos != nil {
		runner.SetResolver(orchestrator.NewServiceScriptResolver(repos, "/usr/local/Wywy-Website"))
	}

	mux := http.NewServeMux()
	h := &Handler{
		Store:            st,
		Runner:           runner,
		EventBroadcaster: eb,
	}
	h.RegisterRoutes(mux)
	srv = httptest.NewServer(mux)

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)

	u := "ws" + srv.URL[4:] + "/api/events"
	conn, _, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		cancel()
		srv.Close()
		t.Fatalf("WebSocket dial: %v", err)
	}

	return srv, conn, ctx, cancel
}
