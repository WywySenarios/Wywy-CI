package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"wywy-ci/apps/mcp"
	"wywy-ci/internal/config"
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

	// Load config from .wywy-ci files (home + project, layered).
	homeConfig := os.Getenv("HOME") + "/.wywy-ci"
	projectConfig := ".wywy-ci"

	cfg, resolver, err := loadConfigOrDie(homeConfig, projectConfig)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Set up config loader for MCP tools.
	mcp.SetConfigLoader(func() (*config.Config, error) {
		return cfg, nil
	})

	// Build set of valid service names from config.
	validServices := make(map[string]bool)
	for _, repo := range resolver.ListServices() {
		validServices[repo] = true
	}

	// Create broadcasters, runner, and handler.
	broadcaster := api.NewBroadcaster()
	eventBroadcaster := api.NewEventBroadcaster()
	runner := orchestrator.NewRunner(st, orchestrator.DefaultRunner)
	runner.SetBroadcaster(broadcaster)
	runner.SetEventBroadcaster(api.NewEventBroadcasterAdapter(eventBroadcaster))
	runner.RunsDir = "/var/lib/Wywy-Website/ci/runs"
	runner.SetResolver(resolver)

	handler := &api.Handler{
		Store:            st,
		Runner:           runner,
		ValidServices:    validServices,
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

// loadConfigOrDie loads config from .wywy-ci files and constructs a resolver.
func loadConfigOrDie(paths ...string) (*config.Config, *orchestrator.ServiceScriptResolver, error) {
	cfg, err := config.Load(paths...)
	if err != nil {
		return nil, nil, err
	}

	services := make(map[string]string)
	for _, r := range cfg.Repos {
		services[r.Name] = r.Path
	}

	return cfg, orchestrator.NewServiceScriptResolver(services, ""), nil
}


