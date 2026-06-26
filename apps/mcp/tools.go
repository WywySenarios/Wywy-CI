package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"wywy-website/ci/apps/testrunner"
)

const (
	reposBasePath       = "/usr/local/Wywy-Website"
	defaultServicesPath = "/etc/Wywy-Website-Control/services.txt"
)

// serviceConfig represents a single entry from services.txt.
type serviceConfig struct {
	Alias string
	Repo  string
}

// loadServices reads services.txt and returns the parsed entries.
// It respects the CI_SERVICES_PATH environment variable override.
func loadServices() ([]serviceConfig, error) {
	path := defaultServicesPath
	if env := os.Getenv("CI_SERVICES_PATH"); env != "" {
		path = env
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var services []serviceConfig
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ",", 2)
		cfg := serviceConfig{Alias: parts[0]}
		if len(parts) > 1 {
			cfg.Repo = parts[1]
		}
		services = append(services, cfg)
	}
	return services, nil
}

// loadServicesCached returns the cached services config, loading it once on
// the first call. The config file does not change at runtime.
var loadServicesCached = sync.OnceValues(func() ([]serviceConfig, error) {
	return loadServices()
})

// parseArgs returns the arguments map from a tool request, or an error result
// if the arguments are missing or not a map.
func parseArgs(req mcp.CallToolRequest) (map[string]any, *mcp.CallToolResult) {
	args, ok := req.Params.Arguments.(map[string]any)
	if !ok {
		return nil, mcp.NewToolResultError("invalid arguments")
	}
	return args, nil
}

// parseServiceArg extracts the required "service" argument from a tool request.
// It returns the service name, the full arguments map (for further extraction),
// and an error result if the argument is missing or invalid.
func parseServiceArg(req mcp.CallToolRequest) (string, map[string]any, *mcp.CallToolResult) {
	args, errResult := parseArgs(req)
	if errResult != nil {
		return "", nil, errResult
	}
	service, ok := args["service"].(string)
	if !ok || service == "" {
		return "", nil, mcp.NewToolResultError("missing required argument: service")
	}
	return service, args, nil
}

// parseRunIDArg extracts the required "run_id" argument from a tool request.
// It returns the run ID string and an error result if the argument is missing
// or invalid.
func parseRunIDArg(req mcp.CallToolRequest) (string, *mcp.CallToolResult) {
	args, errResult := parseArgs(req)
	if errResult != nil {
		return "", errResult
	}
	runID, ok := args["run_id"].(string)
	if !ok || runID == "" {
		return "", mcp.NewToolResultError("missing required argument: run_id")
	}
	return runID, nil
}

// findService looks up a service by alias from the cached services config.
// It returns false if the service is not found or if loading the config fails.
func findService(alias string) (serviceConfig, bool) {
	services, err := loadServicesCached()
	if err != nil {
		return serviceConfig{}, false
	}
	for _, svc := range services {
		if svc.Alias == alias {
			return svc, true
		}
	}
	return serviceConfig{}, false
}

// textResult marshals v to JSON and returns a successful CallToolResult with
// the JSON as text content. On marshal error, it returns an error result.
func textResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultError("failed to marshal response: " + err.Error()), nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.TextContent{Type: "text", Text: string(data)}},
	}, nil
}

// runResultResponse converts a testrunner.RunResult into a JSON tool response
// with run_id and status fields.
func runResultResponse(result *testrunner.RunResult) (*mcp.CallToolResult, error) {
	return textResult(map[string]string{
		"run_id": result.ID,
		"status": result.Status,
	})
}

// HandleListServices returns all CI services with their repos and available
// test suites as a JSON array of {name, repo, suites} objects.
func HandleListServices(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	services, err := loadServicesCached()
	if err != nil {
		return mcp.NewToolResultError("failed to load services: " + err.Error()), nil
	}

	type entry struct {
		Name   string   `json:"name"`
		Repo   string   `json:"repo"`
		Suites []string `json:"suites"`
	}

	var result []entry
	for _, svc := range services {
		suites, _ := testrunner.ListTestSuites(reposBasePath, svc.Repo)
		if suites == nil {
			suites = []string{}
		}
		result = append(result, entry{
			Name:   svc.Alias,
			Repo:   svc.Repo,
			Suites: suites,
		})
	}

	return textResult(result)
}

// HandleListTestFiles returns the test files for a given service, optionally
// filtered by suite, as a JSON array of {path, suite} objects.
func HandleListTestFiles(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	service, args, errResult := parseServiceArg(req)
	if errResult != nil {
		return errResult, nil
	}

	svc, ok := findService(service)
	if !ok {
		return mcp.NewToolResultError("unknown service: " + service), nil
	}

	pattern := filepath.Join(reposBasePath, svc.Repo, "scripts/tests", "*.sh")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return mcp.NewToolResultError("failed to list test files: " + err.Error()), nil
	}

	suiteFilter, hasFilter := args["suite"].(string)

	type entry struct {
		Path  string `json:"path"`
		Suite string `json:"suite"`
	}

	var result []entry
	for _, match := range matches {
		suite := strings.TrimSuffix(filepath.Base(match), ".sh")
		if hasFilter && suite != suiteFilter {
			continue
		}
		result = append(result, entry{Path: match, Suite: suite})
	}

	return textResult(result)
}

// HandleListTestSuites returns the test suites for a given service as a JSON
// array of {name, framework} objects.
func HandleListTestSuites(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	service, _, errResult := parseServiceArg(req)
	if errResult != nil {
		return errResult, nil
	}

	svc, ok := findService(service)
	if !ok {
		return mcp.NewToolResultError("unknown service: " + service), nil
	}

	suites, err := testrunner.ListTestSuites(reposBasePath, svc.Repo)
	if err != nil {
		return mcp.NewToolResultError("failed to list suites: " + err.Error()), nil
	}

	type entry struct {
		Name      string `json:"name"`
		Framework string `json:"framework"`
	}

	var result []entry
	for _, s := range suites {
		result = append(result, entry{Name: s, Framework: "shell"})
	}

	return textResult(result)
}

// HandleRunTests handles the run_tests tool call.
func HandleRunTests(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	service, args, errResult := parseServiceArg(req)
	if errResult != nil {
		return errResult, nil
	}
	if _, ok := findService(service); !ok {
		return mcp.NewToolResultError("unknown service: " + service), nil
	}
	suite, _ := args["suite"].(string)

	result, err := testrunner.RunTests(service, suite)
	if err != nil {
		return mcp.NewToolResultError("failed to start run: " + err.Error()), nil
	}

	return runResultResponse(result)
}

// HandleRunTargetedTest handles the run_targeted_test tool call.
func HandleRunTargetedTest(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, ok := req.Params.Arguments.(map[string]any)
	if !ok {
		return mcp.NewToolResultError("invalid arguments"), nil
	}
	service, ok := args["service"].(string)
	if !ok || service == "" {
		return mcp.NewToolResultError("missing required argument: service"), nil
	}
	if _, ok := findService(service); !ok {
		return mcp.NewToolResultError("unknown service: " + service), nil
	}
	target, ok := args["target"].(string)
	if !ok || target == "" {
		return mcp.NewToolResultError("missing required argument: target"), nil
	}
	targetTypeStr, ok := args["target_type"].(string)
	if !ok || targetTypeStr == "" {
		return mcp.NewToolResultError("missing required argument: target_type"), nil
	}
	var targetType testrunner.TargetType
	switch targetTypeStr {
	case "file":
		targetType = testrunner.TargetFile
	case "test_name":
		targetType = testrunner.TargetTestName
	case "pattern":
		targetType = testrunner.TargetPattern
	default:
		return mcp.NewToolResultError("invalid target_type: must be one of: file, test_name, pattern"), nil
	}
	suite, _ := args["suite"].(string)

	result, err := testrunner.RunTargetedTest(service, target, targetType, suite)
	if err != nil {
		return mcp.NewToolResultError("failed to start run: " + err.Error()), nil
	}

	return runResultResponse(result)
}

// HandleCancelRun handles the cancel_run tool call.
func HandleCancelRun(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	runID, errResult := parseRunIDArg(req)
	if errResult != nil {
		return errResult, nil
	}

	if err := testrunner.CancelRun(runID); err != nil {
		return mcp.NewToolResultError("failed to cancel run: " + err.Error()), nil
	}

	return textResult(map[string]string{
		"status": "cancelled",
	})
}

// HandleGetRunStatus handles the get_run_status tool call.
func HandleGetRunStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	runID, errResult := parseRunIDArg(req)
	if errResult != nil {
		return errResult, nil
	}

	status, err := testrunner.GetRunStatus(runID)
	if err != nil {
		return mcp.NewToolResultError("failed to get run status: " + err.Error()), nil
	}

	return textResult(map[string]any{
		"run_id":           status.ID,
		"status":           status.Status,
		"running_services": status.RunningServices,
	})
}

// HandleGetRunResults handles the get_run_results tool call.
func HandleGetRunResults(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	runID, errResult := parseRunIDArg(req)
	if errResult != nil {
		return errResult, nil
	}

	results, err := testrunner.GetRunResults(runID)
	if err != nil {
		return mcp.NewToolResultError("failed to get run results: " + err.Error()), nil
	}

	type serviceEntry struct {
		Name    string `json:"name"`
		Status  string `json:"status"`
		Passed  int    `json:"passed"`
		Failed  int    `json:"failed"`
		Skipped int    `json:"skipped"`
		LogRef  string `json:"log_ref"`
	}

	services := make([]serviceEntry, len(results.Services))
	for i, s := range results.Services {
		services[i] = serviceEntry{
			Name:    s.Name,
			Status:  s.Status,
			Passed:  s.Passed,
			Failed:  s.Failed,
			Skipped: s.Skipped,
			LogRef:  s.LogRef,
		}
	}

	return textResult(map[string]any{
		"run_id":   results.ID,
		"status":   results.Status,
		"services": services,
	})
}
