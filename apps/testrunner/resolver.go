package testrunner

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ServiceScriptResolver resolves script paths from service names.
type ServiceScriptResolver struct {
	services      map[string]string // service alias → repo name
	reposBasePath string
}

// NewServiceScriptResolver creates a resolver with a service→repo mapping.
func NewServiceScriptResolver(services map[string]string, reposBasePath string) *ServiceScriptResolver {
	return &ServiceScriptResolver{services: services, reposBasePath: reposBasePath}
}

// ResolveScriptPath returns the path to a test script for the given service and suite.
func (r *ServiceScriptResolver) ResolveScriptPath(service, suite string) (string, error) {
	repo, ok := r.services[service]
	if !ok {
		return "", fmt.Errorf("unknown service: %s", service)
	}
	return fmt.Sprintf("%s/%s/scripts/tests/%s.sh", r.reposBasePath, repo, suite), nil
}

// ListSuites returns the available test suite names for a service by listing
// *.sh files in the service's scripts/tests/ directory.
func (r *ServiceScriptResolver) ListSuites(service string) ([]string, error) {
	repo, ok := r.services[service]
	if !ok {
		return nil, fmt.Errorf("unknown service: %s", service)
	}
	suites, err := ListTestSuites(r.reposBasePath, repo)
	if err != nil {
		return nil, fmt.Errorf("list suites for %s: %w", service, err)
	}
	return suites, nil
}

// ListTestSuites returns the available test suite names by globbing *.sh files
// in the given repo's scripts/tests/ directory.
func ListTestSuites(reposBasePath, repo string) ([]string, error) {
	pattern := filepath.Join(reposBasePath, repo, "scripts/tests", "*.sh")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var suites []string
	for _, match := range matches {
		name := strings.TrimSuffix(filepath.Base(match), ".sh")
		if !seen[name] {
			seen[name] = true
			suites = append(suites, name)
		}
	}
	return suites, nil
}
