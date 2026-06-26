package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"wywy-ci/apps/mcp"
	"wywy-ci/server/api"
	"wywy-ci/server/orchestrator"
	"wywy-ci/server/store"
)

func main() {
	// Open SQLite store.
	dbPath := os.Getenv("CI_DB_PATH")
	if dbPath == "" {
		dbPath = "/var/lib/Wywy-Website/ci/ci.db"
	}

	st, err := openStore(dbPath)
	if err != nil {
		log.Fatalf("failed to open store at %s: %v", dbPath, err)
	}
	defer st.Close()

	// Load valid services from services.txt (optional).
	servicesPath := os.Getenv("CI_SERVICES_PATH")
	if servicesPath == "" {
		servicesPath = "/etc/Wywy-Website-Control/services.txt"
	}

	validServices, serviceRepoMap := loadServices(servicesPath)

	// Create broadcasters, runner, and handler.
	broadcaster := api.NewBroadcaster()
	eventBroadcaster := api.NewEventBroadcaster()
	runner := orchestrator.NewRunner(st, orchestrator.DefaultRunner)
	runner.SetBroadcaster(broadcaster)
	runner.SetEventBroadcaster(api.NewEventBroadcasterAdapter(eventBroadcaster))
	runner.RunsDir = "/var/lib/Wywy-Website/ci/runs"
	if serviceRepoMap != nil {
		runner.SetResolver(orchestrator.NewServiceScriptResolver(
			serviceRepoMap, "/usr/local/Wywy-Website",
		))
	}

	handler := &api.Handler{
		Store:            st,
		Runner:           runner,
		ValidServices:    validServices,
		ServicesPath:     servicesPath,
		Broadcaster:      broadcaster,
		EventBroadcaster: eventBroadcaster,
	}

	// Start MCP server in a goroutine.
	mcpSrv := mcp.NewMCPServer("Wywy-CI", "1.0.0")
	if err := mcpSrv.RegisterDefaults(); err != nil {
		log.Fatalf("MCP server registration failed: %v", err)
	}
	mcpTransport := mcp.NewTransport(mcpSrv.HTTPHandler())
	go func() {
		lis, err := net.Listen("tcp", mcpTransport.Addr)
		if err != nil {
			log.Fatalf("MCP server failed to listen on %s: %v", mcpTransport.Addr, err)
		}
		log.Printf("Wywy-CI MCP server listening on %s", mcpTransport.Addr)
		if err := mcpTransport.Serve(lis); err != nil && err != http.ErrServerClosed {
			log.Fatalf("MCP server error: %v", err)
		}
	}()

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

// openStore opens the SQLite store at the given path.
// It does NOT fall back to :memory: — callers must handle the error.
func openStore(path string) (*store.Store, error) {
	return store.Open(path)
}

// loadServices reads the services.txt file (name,repo format) and returns
// a set of valid service names and a map of service name → repo name.
// If the file cannot be read, both return values are nil (skip validation).
func loadServices(path string) (map[string]bool, map[string]string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}

	services := make(map[string]bool)
	repos := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ",", 2)
		name := parts[0]
		services[name] = true
		if len(parts) > 1 {
			repos[name] = parts[1]
		}
	}
	return services, repos
}
