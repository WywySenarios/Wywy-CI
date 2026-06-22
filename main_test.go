package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadServicesParsesNameBeforeComma(t *testing.T) {
	// Create a temporary services.txt with "name,repo" lines.
	dir := t.TempDir()
	path := filepath.Join(dir, "services.txt")
	content := []byte("cache,Wywy-Website-Cache\nci,Wywy-CI\nagentic,Wywy-Codes\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write services.txt: %v", err)
	}

	services, repos := loadServices(path)

	// Should contain the short names (first column), not the full lines.
	if !services["ci"] {
		t.Error(`services["ci"]: want true, got false`)
	}
	if !services["cache"] {
		t.Error(`services["cache"]: want true, got false`)
	}
	if !services["agentic"] {
		t.Error(`services["agentic"]: want true, got false`)
	}

	// Should NOT contain the full "repo" entries as keys.
	if services["ci,Wywy-CI"] {
		t.Error(`services["ci,Wywy-CI"]: want false, got true — full line was stored as key`)
	}
	if services["agentic,Wywy-Codes"] {
		t.Error(`services["agentic,Wywy-Codes"]: want false, got true — full line was stored as key`)
	}

	// Repo map should map names to repos correctly.
	if repos["ci"] != "Wywy-CI" {
		t.Errorf(`repos["ci"]: want "Wywy-CI", got %q`, repos["ci"])
	}
	if repos["cache"] != "Wywy-Website-Cache" {
		t.Errorf(`repos["cache"]: want "Wywy-Website-Cache", got %q`, repos["cache"])
	}
}

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

func TestLoadServicesReturnsNilOnMissingFile(t *testing.T) {
	services, repos := loadServices("/nonexistent/path")
	if services != nil {
		t.Errorf("loadServices with missing file: want nil, got %v", services)
	}
	if repos != nil {
		t.Errorf("loadServices with missing file: repos want nil, got %v", repos)
	}
}
