package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestListServices(t *testing.T) {
	s := newTestStore(t)

	// Create a temp services.txt mirroring the production format.
	content := []byte(
		"agentic,Wywy-Codes\n" +
			"cache,Wywy-Website-Cache\n" +
			"master-database,Wywy-Website-Master-Database\n" +
			"website,Wywy-Website\n" +
			"ci,Wywy-CI,2526",
	)
	path := filepath.Join(t.TempDir(), "services.txt")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write services file: %v", err)
	}

	mux := http.NewServeMux()
	h := &Handler{
		Store:        s,
		ServicesPath: path,
	}
	h.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/services")
	if err != nil {
		t.Fatalf("GET /api/services: %v", err)
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

	// Verify at minimum the four core services exist.
	names := make(map[string]bool)
	for _, svc := range result {
		name, ok := svc["name"].(string)
		if !ok || name == "" {
			t.Error("service missing 'name'")
			continue
		}
		names[name] = true

		if _, ok := svc["repo"]; !ok {
			t.Errorf("service %q missing 'repo'", name)
		}
	}

	for _, want := range []string{"agentic", "cache", "master-database", "website"} {
		if !names[want] {
			t.Errorf("missing service: %s", want)
		}
	}
}
