package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	wywymcp "wywy-ci/apps/mcp"
	"wywy-ci/internal/config"
)

func TestOpenStoreReturnsErrorOnFail(t *testing.T) {
	// Create a scenario where store.Open fails: make a file where
	// MkdirAll expects a directory, so the dir-creation step fails.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(blocker, []byte{}, 0644); err != nil {
		t.Fatalf("write blocker file: %v", err)
	}
	badPath := filepath.Join(blocker, "ci.db")

	// openStore must NOT fall back — it should return the error.
	_, err := openStore(badPath)
	if err == nil {
		t.Fatal("expected error when database path is inaccessible, got nil")
	}
}

func TestLoadConfigCreatesResolver(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".wywy-ci")

	repoPath := "/usr/local/Wywy-Website/Wywy-CI"
	cfg := fmt.Sprintf(`{"repos":[{"name":"ci","path":"%s"}]}`, repoPath)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0644); err != nil {
		t.Fatalf("write .wywy-ci: %v", err)
	}

	_, resolver, err := loadConfigOrDie(cfgPath)
	if err != nil {
		t.Fatalf("loadConfigOrDie: want nil error, got %v", err)
	}
	if resolver == nil {
		t.Fatal("loadConfigOrDie: want non-nil resolver, got nil")
	}

	got, err := resolver.ResolveScriptPath("ci", "unit")
	if err != nil {
		t.Fatalf("ResolveScriptPath: want nil error, got %v", err)
	}

	wantSuffix := "Wywy-CI/scripts/tests/unit.sh"
	if !strings.HasSuffix(got, wantSuffix) {
		t.Errorf("ResolveScriptPath path: want suffix %q, got %q", wantSuffix, got)
	}
}

func TestLoadConfigErrorsOnNoConfigFiles(t *testing.T) {
	dir := t.TempDir()

	_, _, err := loadConfigOrDie(filepath.Join(dir, "nonexistent"))
	if err == nil {
		t.Error("loadConfigOrDie with nonexistent path: want error, got nil")
	}
}

func getTextContent(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	tc, ok := mcp.AsTextContent(result.Content[0])
	if !ok {
		t.Fatal("expected text content")
	}
	return tc.Text
}

func TestMainSetsMCPConfigLoader(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".wywy-ci")

	cfgContent := fmt.Sprintf(`{"repos":[{"name":"test-service","path":"%s"}]}`, dir)
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatalf("write .wywy-ci: %v", err)
	}

	cfg, _, err := loadConfigOrDie(cfgPath)
	if err != nil {
		t.Fatalf("loadConfigOrDie: want nil error, got %v", err)
	}

	// Simulate what main() does: set the config loader so MCP handlers see the config.
	wywymcp.SetConfigLoader(func() (*config.Config, error) {
		return cfg, nil
	})
	t.Cleanup(func() { wywymcp.SetConfigLoader(nil) })

	result, err := wywymcp.HandleListServices(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("HandleListServices: unexpected transport error: %v", err)
	}
	if result.IsError {
		t.Fatalf("HandleListServices returned error: %s", mcp.GetTextFromContent(result.Content[0]))
	}

	text := getTextContent(t, result)
	var services []struct {
		Name   string   `json:"name"`
		Repo   string   `json:"repo"`
		Suites []string `json:"suites"`
	}
	if err := json.Unmarshal([]byte(text), &services); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	if services[0].Name != "test-service" {
		t.Errorf("expected service name 'test-service', got %q", services[0].Name)
	}
}


