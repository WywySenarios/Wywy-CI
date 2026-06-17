package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"wywy-website/ci/server/orchestrator"

	"wywy-website/ci/server/store"
)

// fakeRunner returns a fixed exit code and output for API tests.
type fakeRunner struct {
	exitCode int
	output   string
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) (int, string, error) {
	return f.exitCode, f.output, nil
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

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status: want 201, got %d", resp.StatusCode)
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

// newTestServer creates an httptest.Server with the given store and runner.
func newTestServer(t *testing.T, st *store.Store, runner *orchestrator.Runner, validServices map[string]bool) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	h := &Handler{Store: st, Runner: runner, ValidServices: validServices}
	h.RegisterRoutes(mux)
	return httptest.NewServer(mux)
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
