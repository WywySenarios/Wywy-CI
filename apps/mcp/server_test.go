package mcp_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"wywy-ci/apps/mcp"
)

// TestHTTPHandler_ServesOnAddress proves the HTTP+SSE transport handler
// can be obtained and serves HTTP requests on a listening address.
// TestServer_RegisterDefaults_RegistersAllTools proves that registering all
// default tools results in exactly 8 tools being available on the server.
func TestServer_RegisterDefaults_RegistersAllTools(t *testing.T) {
	s := mcp.NewMCPServer("Wywy-CI", "1.0.0")
	if err := s.RegisterDefaults(); err != nil {
		t.Fatalf("RegisterDefaults failed: %v", err)
	}
	if got := s.ToolCount(); got != 8 {
		t.Fatalf("expected 8 tools after RegisterDefaults, got %d", got)
	}
}

func TestHTTPHandler_ServesOnAddress(t *testing.T) {
	s := mcp.NewMCPServer("test", "1.0.0")
	handler := s.HTTPHandler()
	if handler == nil {
		t.Fatal("HTTPHandler returned nil")
	}

	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	resp.Body.Close()
}
