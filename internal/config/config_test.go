package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".wywy-ci")
	content := `{"repos": [{"name": "ci", "path": "/usr/local/Wywy-Website/Wywy-CI"}, {"name": "cache", "path": "/usr/local/Wywy-Website/Wywy-Website-Cache"}]}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}
	if len(cfg.Repos) != 2 {
		t.Fatalf("len(Repos): want 2, got %d", len(cfg.Repos))
	}
	if cfg.Repos[0].Name != "ci" {
		t.Errorf("Repos[0].Name: want ci, got %q", cfg.Repos[0].Name)
	}
	if cfg.Repos[0].Path != "/usr/local/Wywy-Website/Wywy-CI" {
		t.Errorf("Repos[0].Path: want /usr/local/Wywy-Website/Wywy-CI, got %q", cfg.Repos[0].Path)
	}
	if cfg.Repos[1].Name != "cache" {
		t.Errorf("Repos[1].Name: want cache, got %q", cfg.Repos[1].Name)
	}
	if cfg.Repos[1].Path != "/usr/local/Wywy-Website/Wywy-Website-Cache" {
		t.Errorf("Repos[1].Path: want /usr/local/Wywy-Website/Wywy-Website-Cache, got %q", cfg.Repos[1].Path)
	}
}

func TestMergeHomeAndProjectConfigs(t *testing.T) {
	dir := t.TempDir()

	homePath := filepath.Join(dir, "home.wywy-ci")
	homeContent := `{
		"repos": [
			{"name": "ci", "path": "/home/.wywy/Wywy-CI"},
			{"name": "cache", "path": "/home/.wywy/Wywy-Website-Cache"},
			{"name": "website", "path": "/home/.wywy/Wywy-Website"}
		]
	}`
	if err := os.WriteFile(homePath, []byte(homeContent), 0644); err != nil {
		t.Fatalf("write home config: %v", err)
	}

	projectPath := filepath.Join(dir, "project.wywy-ci")
	projectContent := `{
		"repos": [
			{"name": "ci", "path": "/usr/local/Wywy-Website/Wywy-CI"},
			{"name": "agentic", "path": "/usr/local/Wywy-Website/Wywy-Codes"}
		]
	}`
	if err := os.WriteFile(projectPath, []byte(projectContent), 0644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	cfg, err := Load(homePath, projectPath)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}

	// Expect 4 repos: ci (overridden by project), cache (from home), website (from home), agentic (from project)
	if len(cfg.Repos) != 4 {
		t.Fatalf("len(Repos): want 4, got %d", len(cfg.Repos))
	}

	// ci should have the project path (override)
	found := make(map[string]string)
	for _, r := range cfg.Repos {
		found[r.Name] = r.Path
	}

	if found["ci"] != "/usr/local/Wywy-Website/Wywy-CI" {
		t.Errorf("ci path: want %q, got %q", "/usr/local/Wywy-Website/Wywy-CI", found["ci"])
	}
	if found["cache"] != "/home/.wywy/Wywy-Website-Cache" {
		t.Errorf("cache path: want %q, got %q", "/home/.wywy/Wywy-Website-Cache", found["cache"])
	}
	if found["website"] != "/home/.wywy/Wywy-Website" {
		t.Errorf("website path: want %q, got %q", "/home/.wywy/Wywy-Website", found["website"])
	}
	if found["agentic"] != "/usr/local/Wywy-Website/Wywy-Codes" {
		t.Errorf("agentic path: want %q, got %q", "/usr/local/Wywy-Website/Wywy-Codes", found["agentic"])
	}
}

func TestMissingFilesReturnsError(t *testing.T) {
	_, err := Load("/nonexistent/path/.wywy-ci")
	if err == nil {
		t.Fatal("Load() with nonexistent path: expected error, got nil")
	}
}

func TestInvalidJSONReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.wywy-ci")
	content := `{invalid json here}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() with invalid JSON: expected error, got nil")
	}
}

func TestNonexistentRepoPathIsSkipped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".wywy-ci")
	content := `{"repos": [{"name": "existing", "path": "` + dir + `"}, {"name": "missing", "path": "/nonexistent/path/that/does/not/exist"}]}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}

	foundExisting := false
	foundMissing := false
	for _, r := range cfg.Repos {
		if r.Name == "existing" {
			foundExisting = true
		}
		if r.Name == "missing" {
			foundMissing = true
		}
	}
	if !foundExisting {
		t.Error("expected 'existing' repo to be present, but it was skipped")
	}
	if foundMissing {
		t.Error("expected 'missing' repo (nonexistent path) to be skipped, but it was present")
	}
}

func TestEmptyReposIsValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".wywy-ci")
	content := `{"repos": []}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}
	if cfg.Repos == nil {
		t.Fatal("Load() returned nil Repos slice")
	}
	if len(cfg.Repos) != 0 {
		t.Fatalf("len(Repos): want 0, got %d", len(cfg.Repos))
	}
}
