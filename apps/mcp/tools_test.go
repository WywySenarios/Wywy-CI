package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/mcptest"
	"github.com/mark3labs/mcp-go/server"

	wywy "wywy-ci/apps/mcp"
)

// toolClient is satisfied by the client returned from mcptest.Server.Client().
type toolClient interface {
	CallTool(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error)
}

// callToolSuccess calls a tool and fails the test if the transport or the tool
// itself returns an error.
func callToolSuccess(t *testing.T, client toolClient, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := client.CallTool(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: name, Arguments: args},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatal(name, "returned error:", mcp.GetTextFromContent(result.Content[0]))
	}
	return result
}

// getTextContent extracts the text string from a successful CallToolResult,
// failing the test if the content is not text.
func getTextContent(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	tc, ok := mcp.AsTextContent(result.Content[0])
	if !ok {
		t.Fatal("expected text content")
	}
	return tc.Text
}

func TestListServicesTool_ReturnsServices(t *testing.T) {
	tool := mcp.NewTool("list_services",
		mcp.WithDescription("Lists all available CI services with their repos and test suites"),
	)
	s, err := mcptest.NewServer(t, server.ServerTool{
		Tool:    tool,
		Handler: wywy.HandleListServices,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	result := callToolSuccess(t, s.Client(), "list_services", nil)
	text := getTextContent(t, result)

	var services []struct {
		Name   string   `json:"name"`
		Repo   string   `json:"repo"`
		Suites []string `json:"suites"`
	}
	if err := json.Unmarshal([]byte(text), &services); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if len(services) == 0 {
		t.Fatal("expected at least one service")
	}
}

func TestListTestFilesTool_ReturnsTestFiles(t *testing.T) {
	tool := mcp.NewTool("list_test_files",
		mcp.WithDescription("Lists test files for a given service and optional suite"),
		mcp.WithString("service", mcp.Required(), mcp.Description("Service name")),
		mcp.WithString("suite", mcp.Description("Optional suite name")),
	)
	s, err := mcptest.NewServer(t, server.ServerTool{
		Tool:    tool,
		Handler: wywy.HandleListTestFiles,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	result := callToolSuccess(t, s.Client(), "list_test_files", map[string]any{"service": "ci"})
	text := getTextContent(t, result)

	var files []struct {
		Path  string `json:"path"`
		Suite string `json:"suite"`
	}
	if err := json.Unmarshal([]byte(text), &files); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("expected at least one test file")
	}
}

func TestListTestSuitesTool_ReturnsTestSuites(t *testing.T) {
	tool := mcp.NewTool("list_test_suites",
		mcp.WithDescription("Lists test suites for a given service"),
		mcp.WithString("service", mcp.Required(), mcp.Description("Service name")),
	)
	s, err := mcptest.NewServer(t, server.ServerTool{
		Tool:    tool,
		Handler: wywy.HandleListTestSuites,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	result := callToolSuccess(t, s.Client(), "list_test_suites", map[string]any{"service": "ci"})
	text := getTextContent(t, result)

	var suites []struct {
		Name      string `json:"name"`
		Framework string `json:"framework"`
	}
	if err := json.Unmarshal([]byte(text), &suites); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if len(suites) == 0 {
		t.Fatal("expected at least one suite")
	}
}

func TestRunTestsTool_StartsRun(t *testing.T) {
	tool := mcp.NewTool("run_tests",
		mcp.WithDescription("Runs all tests for a service, optionally scoped to a suite"),
		mcp.WithString("service", mcp.Required(), mcp.Description("Service name")),
		mcp.WithString("suite", mcp.Description("Suite name (defaults to all suites)")),
	)
	s, err := mcptest.NewServer(t, server.ServerTool{
		Tool:    tool,
		Handler: wywy.HandleRunTests,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	result := callToolSuccess(t, s.Client(), "run_tests", map[string]any{"service": "ci"})
	text := getTextContent(t, result)

	var run struct {
		ID     string `json:"run_id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(text), &run); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if run.ID == "" {
		t.Fatal("expected non-empty run_id")
	}
	if run.Status == "" {
		t.Fatal("expected non-empty status")
	}
}

func TestRunTargetedTestTool_StartsRun(t *testing.T) {
	tool := mcp.NewTool("run_targeted_test",
		mcp.WithDescription("Runs a targeted test by file path, test name, or pattern"),
		mcp.WithString("service", mcp.Required(), mcp.Description("Service name")),
		mcp.WithString("target", mcp.Required(), mcp.Description("Test file path, name, or pattern")),
		mcp.WithString("target_type", mcp.Required(), mcp.Description("One of: file, test_name, pattern")),
		mcp.WithString("suite", mcp.Description("Optional suite name")),
	)
	s, err := mcptest.NewServer(t, server.ServerTool{
		Tool:    tool,
		Handler: wywy.HandleRunTargetedTest,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	result := callToolSuccess(t, s.Client(), "run_targeted_test", map[string]any{
		"service":     "ci",
		"target":      "TestListServices",
		"target_type": "test_name",
	})
	text := getTextContent(t, result)

	var run struct {
		ID     string `json:"run_id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(text), &run); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if run.ID == "" {
		t.Fatal("expected non-empty run_id")
	}
	if run.Status == "" {
		t.Fatal("expected non-empty status")
	}
}

func TestCancelRunTool_CancelsRun(t *testing.T) {
	tool := mcp.NewTool("cancel_run",
		mcp.WithDescription("Cancels a running test run by run ID"),
		mcp.WithString("run_id", mcp.Required(), mcp.Description("Run ID to cancel")),
	)
	s, err := mcptest.NewServer(t, server.ServerTool{
		Tool:    tool,
		Handler: wywy.HandleCancelRun,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	result := callToolSuccess(t, s.Client(), "cancel_run", map[string]any{"run_id": "run-1234567890"})
	text := getTextContent(t, result)

	var cancel struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(text), &cancel); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if cancel.Status != "cancelled" {
		t.Fatalf("expected status 'cancelled', got %q", cancel.Status)
	}
}

func TestGetRunStatusTool_ReturnsRunStatus(t *testing.T) {
	tool := mcp.NewTool("get_run_status",
		mcp.WithDescription("Returns the status of a test run"),
		mcp.WithString("run_id", mcp.Required(), mcp.Description("Run ID to query")),
	)
	s, err := mcptest.NewServer(t, server.ServerTool{
		Tool:    tool,
		Handler: wywy.HandleGetRunStatus,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	result := callToolSuccess(t, s.Client(), "get_run_status", map[string]any{"run_id": "run-1234567890"})
	text := getTextContent(t, result)

	var status struct {
		ID              string   `json:"run_id"`
		Status          string   `json:"status"`
		RunningServices []string `json:"running_services"`
	}
	if err := json.Unmarshal([]byte(text), &status); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if status.ID == "" {
		t.Fatal("expected non-empty run_id")
	}
	if status.Status == "" {
		t.Fatal("expected non-empty status")
	}
}

func TestGetRunResultsTool_ReturnsRunResults(t *testing.T) {
	tool := mcp.NewTool("get_run_results",
		mcp.WithDescription("Returns the detailed results of a test run"),
		mcp.WithString("run_id", mcp.Required(), mcp.Description("Run ID to query")),
	)
	s, err := mcptest.NewServer(t, server.ServerTool{
		Tool:    tool,
		Handler: wywy.HandleGetRunResults,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	result := callToolSuccess(t, s.Client(), "get_run_results", map[string]any{"run_id": "run-1234567890"})
	text := getTextContent(t, result)

	var results struct {
		ID       string `json:"run_id"`
		Status   string `json:"status"`
		Services []struct {
			Name    string `json:"name"`
			Status  string `json:"status"`
			Passed  int    `json:"passed"`
			Failed  int    `json:"failed"`
			Skipped int    `json:"skipped"`
			LogRef  string `json:"log_ref"`
		} `json:"services"`
	}
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if results.ID == "" {
		t.Fatal("expected non-empty run_id")
	}
	if results.Status == "" {
		t.Fatal("expected non-empty status")
	}
}
