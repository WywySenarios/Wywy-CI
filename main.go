package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"wywy-website/ci/server/api"
	"wywy-website/ci/server/orchestrator"
	"wywy-website/ci/server/store"
)

func main() {
	// Open SQLite store.
	dbPath := os.Getenv("CI_DB_PATH")
	if dbPath == "" {
		dbPath = "/var/lib/Wywy-Website/ci/ci.db"
	}

	st, err := store.Open(dbPath)
	if err != nil {
		// Fall back to in-memory for development.
		st, err = store.Open(":memory:")
		if err != nil {
			log.Fatalf("failed to open store: %v", err)
		}
	}
	defer st.Close()

	// Load valid services from services.txt (optional).
	servicesPath := os.Getenv("CI_SERVICES_PATH")
	if servicesPath == "" {
		servicesPath = "/etc/Wywy-Website-Control/services.txt"
	}

	validServices := loadServices(servicesPath)

	// Create broadcaster, runner, and handler.
	broadcaster := api.NewBroadcaster()
	runner := orchestrator.NewRunner(st, orchestrator.DefaultRunner)
	runner.SetBroadcaster(broadcaster)
	runner.LogsDir = "/var/log/Wywy-Website/ci/runs"

	handler := &api.Handler{
		Store:         st,
		Runner:        runner,
		ValidServices: validServices,
		ServicesPath:  servicesPath,
		Broadcaster:   broadcaster,
	}

	// Register routes and wrap with CORS.
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	wrapped := api.CORS(mux)

	addr := ":2526"
	fmt.Printf("Wywy-CI server listening on %s\n", addr)
	if err := http.ListenAndServe(addr, wrapped); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// loadServices reads the services.txt file and returns a set of valid service names.
// If the file cannot be read, it returns nil (skip validation).
func loadServices(path string) map[string]bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	services := make(map[string]bool)
	for _, line := range splitLines(string(data)) {
		if line != "" {
			services[line] = true
		}
	}
	return services
}

// splitLines splits a string by newlines and trims whitespace from each line.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if line != "" {
				lines = append(lines, line)
			}
			start = i + 1
		}
	}
	// Handle last line without trailing newline.
	if start < len(s) {
		line := s[start:]
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
