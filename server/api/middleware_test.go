package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"wywy-website/ci/server/orchestrator"
)

func TestCORSPreflight(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/runs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	})

	wrapped := CORS(mux)
	srv := httptest.NewServer(wrapped)
	defer srv.Close()

	req, err := http.NewRequest("OPTIONS", srv.URL+"/api/runs", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Origin", "http://localhost:3001")
	req.Header.Set("Access-Control-Request-Method", "GET")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: want 200, got %d", resp.StatusCode)
	}

	got := resp.Header.Get("Access-Control-Allow-Origin")
	if got != "*" {
		t.Errorf("Access-Control-Allow-Origin: want *, got %q", got)
	}

	got = resp.Header.Get("Access-Control-Allow-Methods")
	if got != "GET, POST, OPTIONS" {
		t.Errorf("Access-Control-Allow-Methods: want GET, POST, OPTIONS, got %q", got)
	}

	got = resp.Header.Get("Access-Control-Allow-Headers")
	if got != "Content-Type" {
		t.Errorf("Access-Control-Allow-Headers: want Content-Type, got %q", got)
	}
}

func TestCORSAttachesHeadersOnRegularRequests(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/runs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	})

	wrapped := CORS(mux)
	srv := httptest.NewServer(wrapped)
	defer srv.Close()

	req, err := http.NewRequest("GET", srv.URL+"/api/runs", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Origin", "http://localhost:3001")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: want 200, got %d", resp.StatusCode)
	}

	got := resp.Header.Get("Access-Control-Allow-Origin")
	if got != "*" {
		t.Errorf("Access-Control-Allow-Origin: want *, got %q", got)
	}

	// Handler response body must be preserved through CORS wrapper.
	var body []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
}

func TestCORSPassthroughForAPIEndpoints(t *testing.T) {
	s := newTestStore(t)
	runner := orchestrator.NewRunner(s, &fakeRunner{exitCode: 0, output: "ok"})
	mux := http.NewServeMux()
	h := &Handler{Store: s, Runner: runner}
	h.RegisterRoutes(mux)

	wrapped := CORS(mux)
	srv := httptest.NewServer(wrapped)
	defer srv.Close()

	// GET /api/runs — returns empty array.
	resp, err := http.Get(srv.URL + "/api/runs")
	if err != nil {
		t.Fatalf("GET /api/runs: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /api/runs status: want 200, got %d", resp.StatusCode)
	}
	got := resp.Header.Get("Access-Control-Allow-Origin")
	if got != "*" {
		t.Errorf("GET /api/runs CORS header: want *, got %q", got)
	}
	resp.Body.Close()

	// POST /api/runs — creates a run.
	resp, err = http.Post(srv.URL+"/api/runs", "application/json",
		jsonBody(t, map[string]any{"services": []string{"agentic"}, "suite": "test"}))
	if err != nil {
		t.Fatalf("POST /api/runs: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("POST /api/runs status: want 202, got %d", resp.StatusCode)
	}
	got = resp.Header.Get("Access-Control-Allow-Origin")
	if got != "*" {
		t.Errorf("POST /api/runs CORS header: want *, got %q", got)
	}
	resp.Body.Close()
}

// jsonBody returns a reader with the JSON-encoded value.
func jsonBody(t *testing.T, v any) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		t.Fatalf("json.Encode: %v", err)
	}
	return &buf
}
