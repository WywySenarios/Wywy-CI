// Package mcp provides the MCP (Model Context Protocol) server infrastructure
// for AI agents to invoke CI operations.
package mcp

import (
	"net/http"

	"github.com/mark3labs/mcp-go/mcp"
	mcp_server "github.com/mark3labs/mcp-go/server"
)

// Server wraps the MCP server lifecycle and tool registration.
// Not thread-safe — must not call RegisterDefaults concurrently with other
// methods.
type Server struct {
	inner *mcp_server.MCPServer
}

// NewMCPServer creates a new MCP server with the given name and version.
func NewMCPServer(name, version string) *Server {
	return &Server{
		inner: mcp_server.NewMCPServer(name, version),
	}
}

// RegisterDefaults registers all 8 CI tools (discovery, execution, results)
// with their descriptions, parameter schemas, and handlers on the server.
func (s *Server) RegisterDefaults() error {
	s.inner.AddTool(
		mcp.NewTool("list_services",
			mcp.WithDescription("Lists all available CI services with their repos and test suites"),
		),
		HandleListServices,
	)
	s.inner.AddTool(
		mcp.NewTool("list_test_files",
			mcp.WithDescription("Lists test files for a given service and optional suite"),
			mcp.WithString("service", mcp.Required(), mcp.Description("Service name")),
			mcp.WithString("suite", mcp.Description("Optional suite name")),
		),
		HandleListTestFiles,
	)
	s.inner.AddTool(
		mcp.NewTool("list_test_suites",
			mcp.WithDescription("Lists test suites for a given service"),
			mcp.WithString("service", mcp.Required(), mcp.Description("Service name")),
		),
		HandleListTestSuites,
	)
	s.inner.AddTool(
		mcp.NewTool("run_tests",
			mcp.WithDescription("Runs all tests for a service, optionally scoped to a suite"),
			mcp.WithString("service", mcp.Required(), mcp.Description("Service name")),
			mcp.WithString("suite", mcp.Description("Suite name (defaults to all suites)")),
		),
		HandleRunTests,
	)
	s.inner.AddTool(
		mcp.NewTool("run_targeted_test",
			mcp.WithDescription("Runs a targeted test by file path, test name, or pattern"),
			mcp.WithString("service", mcp.Required(), mcp.Description("Service name")),
			mcp.WithString("target", mcp.Required(), mcp.Description("Test file path, name, or pattern")),
			mcp.WithString("target_type", mcp.Required(), mcp.Description("One of: file, test_name, pattern")),
			mcp.WithString("suite", mcp.Description("Optional suite name")),
		),
		HandleRunTargetedTest,
	)
	s.inner.AddTool(
		mcp.NewTool("cancel_run",
			mcp.WithDescription("Cancels a running test run by run ID"),
			mcp.WithString("run_id", mcp.Required(), mcp.Description("Run ID to cancel")),
		),
		HandleCancelRun,
	)
	s.inner.AddTool(
		mcp.NewTool("get_run_status",
			mcp.WithDescription("Returns the status of a test run"),
			mcp.WithString("run_id", mcp.Required(), mcp.Description("Run ID to query")),
		),
		HandleGetRunStatus,
	)
	s.inner.AddTool(
		mcp.NewTool("get_run_results",
			mcp.WithDescription("Returns the detailed results of a test run"),
			mcp.WithString("run_id", mcp.Required(), mcp.Description("Run ID to query")),
		),
		HandleGetRunResults,
	)
	return nil
}

// ToolCount returns the number of registered tools.
// Not thread-safe.
func (s *Server) ToolCount() int {
	return len(s.inner.ListTools())
}

// HTTPHandler returns an http.Handler that serves the MCP protocol
// over HTTP+SSE (Streamable HTTP) transport.
// Not thread-safe.
func (s *Server) HTTPHandler() http.Handler {
	return mcp_server.NewStreamableHTTPServer(s.inner)
}
