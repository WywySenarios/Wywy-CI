package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"wywy-ci/apps/testrunner"
)

func TestResolveScriptPath(t *testing.T) {
	resolver := NewServiceScriptResolver(
		map[string]string{"ci": "Wywy-CI"},
		"/usr/local/Wywy-Website",
	)

	// Known service → expected path.
	path, err := resolver.ResolveScriptPath("ci", "test")
	if err != nil {
		t.Fatalf("ResolveScriptPath(ci, test): %v", err)
	}
	expected := "/usr/local/Wywy-Website/Wywy-CI/scripts/tests/test.sh"
	if path != expected {
		t.Errorf("path: want %q, got %q", expected, path)
	}

	// Unknown service → error.
	_, err = resolver.ResolveScriptPath("nonexistent", "test")
	if err == nil {
		t.Error("ResolveScriptPath(nonexistent, test): expected error, got nil")
	}
}

func TestBuildScriptArgs(t *testing.T) {
	inv := testrunner.ScriptInvocation{
		RunID:      "run-abc",
		OutputDir:  "/tmp/runs/run-abc",
		Machine:    true,
		ExtraFlags: []string{"--verbose"},
	}

	args := testrunner.BuildScriptArgs(inv)

	expected := []string{"--run-id=run-abc", "--output-dir=/tmp/runs/run-abc", "--machine", "--verbose"}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, want := range expected {
		if args[i] != want {
			t.Errorf("arg[%d]: want %q, got %q", i, want, args[i])
		}
	}
}

func TestExecuteRequiresResolver(t *testing.T) {
	var buf bytes.Buffer
	ctx := context.Background()

	_, err := Execute(ctx, "agentic", "test", &buf)
	if err == nil {
		t.Fatal("expected error when DefaultResolver is nil, got nil")
	}
}

func TestDetachedRunnerSpawnsScript(t *testing.T) {
	script := filepath.Join(t.TempDir(), "sleep.sh")
	err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 30\necho done"), 0755)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	runner := &DetachedRunner{}
	cmd, err := runner.StartDetached(ctx, t.TempDir(), "sh", script)
	if err != nil {
		t.Fatalf("StartDetached: %v", err)
	}

	// Must return a running process.
	if cmd.Process == nil {
		t.Fatal("cmd.Process is nil — process did not start")
	}

	// Must be in a different process group (detached).
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		t.Fatalf("Getpgid: %v", err)
	}
	if pgid == syscall.Getpgrp() {
		t.Error("expected child process group to differ from parent")
	}

	// Clean up.
	cmd.Process.Signal(syscall.SIGKILL)
	cmd.Wait()
}

func TestMonitorScriptCompletion(t *testing.T) {
	outputDir := t.TempDir()
	scriptDir := t.TempDir()

	// Script writes stdout.log, stderr.log, and results.jsonl after a short sleep.
	scriptContent := fmt.Sprintf(`#!/bin/sh
sleep 1
echo "line from stdout" > %s/stdout.log
echo "line from stderr" > %s/stderr.log
printf '{"name":"alpha","status":"passed"}\n{"name":"beta","status":"failed"}\n' > %s/results.jsonl
`, outputDir, outputDir, outputDir)

	script := filepath.Join(scriptDir, "script.sh")
	if err := os.WriteFile(script, []byte(scriptContent), 0755); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	runner := &DetachedRunner{}
	cmd, err := runner.StartDetached(ctx, outputDir, "sh", script)
	if err != nil {
		t.Fatalf("StartDetached: %v", err)
	}
	defer cmd.Wait()
	defer cmd.Process.Signal(syscall.SIGKILL)

	results, stdout, stderr, err := testrunner.MonitorScriptOutput(ctx, outputDir, 5*time.Second)
	if err != nil {
		t.Fatalf("MonitorScriptOutput: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Name != "alpha" || results[0].Status != "passed" {
		t.Errorf("result[0] = %+v", results[0])
	}
	if results[1].Name != "beta" || results[1].Status != "failed" {
		t.Errorf("result[1] = %+v", results[1])
	}
	if stdout != "line from stdout\n" {
		t.Errorf("stdout = %q", stdout)
	}
	if stderr != "line from stderr\n" {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestMonitorScriptCompletionTimeout(t *testing.T) {
	outputDir := t.TempDir()
	ctx := context.Background()

	_, _, _, err := testrunner.MonitorScriptOutput(ctx, outputDir, 50*time.Millisecond)
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

// TestExecuteResolvesAndRunsTestScript verifies that Execute resolves the
// service+suite to a real test script and runs it — not the stub echo.
func TestExecuteResolvesAndRunsTestScript(t *testing.T) {
	// Create a temp script at the path the resolver will produce.
	tmpDir := t.TempDir()
	scriptDir := filepath.Join(tmpDir, "Wywy-CI", "scripts", "tests")
	if err := os.MkdirAll(scriptDir, 0755); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(scriptDir, "test.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho 'real output'\n"), 0755); err != nil {
		t.Fatal(err)
	}

	// Configure a resolver that maps "ci" -> "Wywy-CI".
	origResolver := DefaultResolver
	DefaultResolver = NewServiceScriptResolver(
		map[string]string{"ci": "Wywy-CI"},
		tmpDir,
	)
	t.Cleanup(func() { DefaultResolver = origResolver })

	// Replace DefaultRunner with a recording runner.
	origRunner := DefaultRunner
	recorder := &recordingCmdRunner{Output: "real test output\n"}
	DefaultRunner = recorder
	t.Cleanup(func() { DefaultRunner = origRunner })

	var buf bytes.Buffer
	ctx := context.Background()

	code, err := Execute(ctx, "ci", "test", &buf)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code: want 0, got %d", code)
	}

	if recorder.Cmd == "" {
		t.Fatal("DefaultRunner.Run was not called")
	}

	// Execute should have resolved the script path and passed it to the runner.
	if !strings.Contains(recorder.Cmd, scriptPath) {
		t.Errorf("Execute should run the resolved script at %q; recorded command: %s", scriptPath, recorder.Cmd)
	}

	// Output should not be the old stub placeholder.
	if strings.Contains(buf.String(), "test output for") {
		t.Errorf("Execute returned stub placeholder output; got: %s", strings.TrimSpace(buf.String()))
	}
}

// TestDefaultRunnerIsNotNil verifies that DefaultRunner is initialized.
func TestDefaultRunnerIsNotNil(t *testing.T) {
	if DefaultRunner == nil {
		t.Error("DefaultRunner should not be nil")
	}
}

// TestDefaultRunnerImplementsDetachedCommandRunner verifies that DefaultRunner satisfies
// the DetachedCommandRunner interface. Without this, the Runner cannot take the
// detached execution path and falls through to the "no execution path" error.
func TestDefaultRunnerImplementsDetachedCommandRunner(t *testing.T) {
	if _, ok := DefaultRunner.(DetachedCommandRunner); !ok {
		t.Error("DefaultRunner must implement DetachedCommandRunner")
	}
}

// TestDetachedRunnerRunWithSuccess verifies that Run executes a command synchronously
// and returns its output and a zero exit code.
func TestDetachedRunnerRunWithSuccess(t *testing.T) {
	dr := &DetachedRunner{}
	ctx := context.Background()
	code, output, err := dr.Run(ctx, "echo", "hello world")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code: want 0, got %d", code)
	}
	if strings.TrimSpace(output) != "hello world" {
		t.Errorf("output: want %q, got %q", "hello world", strings.TrimSpace(output))
	}
}

// TestDetachedRunnerRunWithFailure verifies that Run returns a non-zero exit code
// when the command exits with a failure status.
func TestDetachedRunnerRunWithFailure(t *testing.T) {
	dr := &DetachedRunner{}
	ctx := context.Background()
	code, output, err := dr.Run(ctx, "sh", "-c", "exit 42")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if code != 42 {
		t.Errorf("exit code: want 42, got %d", code)
	}
	// Output is empty for a simple exit.
	if output != "" {
		t.Errorf("output: want empty, got %q", output)
	}
}

// findRepoRoot walks up from the working directory to find the repo root (where go.mod lives).
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("reached filesystem root without finding go.mod")
		}
		dir = parent
	}
}

// TestCITestRunnerRunsPlaywrightE2E verifies that the CI's own test runner
// script (scripts/tests/test.sh) runs Playwright E2E tests. Without this, the
// Astro E2E tests (tests/e2e/) are never executed in CI — they pass locally
// with `npx playwright test` but are invisible to the pipeline.
func TestCITestRunnerRunsPlaywrightE2E(t *testing.T) {
	repoRoot := findRepoRoot(t)
	scriptPath := filepath.Join(repoRoot, "scripts", "tests", "test.sh")

	content, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read test runner script at %s: %v", scriptPath, err)
	}
	script := string(content)

	// The runner script must invoke Playwright E2E tests via a dedicated function.
	if !strings.Contains(script, "run_playwright") && !strings.Contains(script, "playwright test") {
		t.Error("test runner script (scripts/tests/test.sh) must include a call to run Playwright E2E tests; " +
			"expected a run_playwright_e2e function that invokes 'npx playwright test'")
	}

	// The runner script must report Playwright E2E results in results.jsonl.
	if !strings.Contains(script, "playwright-e2e") {
		t.Error("test runner script must write a 'playwright-e2e' entry to results.jsonl " +
			"so the CI runner can ingest the result")
	}
}

// TestSuiteDiscovery verifies that ListSuites correctly discovers available
// test suites by globbing scripts/tests/*.sh in the service's repository,
// and that ResolveScriptPath returns the correct path for each suite.
// It also verifies that every discovered script is compliant with the CI
// runner contract (handles --output-dir=, writes results.jsonl).
// This test is service-agnostic — it creates a temporary directory with
// test scripts and does not depend on any specific service repository.
// It does not need modification as new services or suites are added.
func TestSuiteDiscovery(t *testing.T) {
	repoBase := t.TempDir()
	scriptsDir := filepath.Join(repoBase, "Wywy-Test-Service", "scripts", "tests")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create several suite scripts — each is CI-runner compliant
	// (parses --output-dir=, writes results.jsonl with a named entry).
	suites := []struct {
		name    string
		content string
	}{
		{
			name: "integration",
			content: `#!/bin/sh
output_dir=""
for arg in "$@"; do
  case "$arg" in
    --output-dir=*) output_dir="${arg#*=}" ;;
  esac
done
echo '{"name":"integration-tests","status":"passed"}' > "$output_dir/results.jsonl"
`,
		},
		{
			name: "unit",
			content: `#!/bin/sh
output_dir=""
for arg in "$@"; do
  case "$arg" in
    --output-dir=*) output_dir="${arg#*=}" ;;
  esac
done
echo '{"name":"unit-tests","status":"passed"}' > "$output_dir/results.jsonl"
`,
		},
		{
			name: "lint",
			content: `#!/bin/sh
output_dir=""
for arg in "$@"; do
  case "$arg" in
    --output-dir=*) output_dir="${arg#*=}" ;;
  esac
done
echo '{"name":"lint-tests","status":"passed"}' > "$output_dir/results.jsonl"
`,
		},
	}
	for _, s := range suites {
		path := filepath.Join(scriptsDir, s.name+".sh")
		if err := os.WriteFile(path, []byte(s.content), 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Create a non-.sh file that must be ignored.
	nonScript := filepath.Join(scriptsDir, "readme.txt")
	if err := os.WriteFile(nonScript, []byte("not a script"), 0644); err != nil {
		t.Fatal(err)
	}

	resolver := NewServiceScriptResolver(
		map[string]string{"test-svc": "Wywy-Test-Service"},
		repoBase,
	)

	// ListSuites should return only .sh files (excluding .txt, etc.).
	got, err := resolver.ListSuites("test-svc")
	if err != nil {
		t.Fatalf("ListSuites: %v", err)
	}

	suiteNames := make([]string, len(suites))
	for i, s := range suites {
		suiteNames[i] = s.name
	}
	if len(got) != len(suiteNames) {
		t.Fatalf("ListSuites: want %d suites, got %d: %v", len(suiteNames), len(got), got)
	}

	want := make(map[string]bool)
	for _, s := range suiteNames {
		want[s] = true
	}
	for _, name := range got {
		if !want[name] {
			t.Errorf("ListSuites: unexpected suite %q", name)
		}
		delete(want, name)
	}
	for name := range want {
		t.Errorf("ListSuites: missing suite %q", name)
	}

	// For every discovered suite, verify resolution and compliance.
	for _, s := range got {
		t.Run(s, func(t *testing.T) {
			path, err := resolver.ResolveScriptPath("test-svc", s)
			if err != nil {
				t.Fatalf("ResolveScriptPath(%q): %v", s, err)
			}
			expected := filepath.Join(scriptsDir, s+".sh")
			if path != expected {
				t.Fatalf("ResolveScriptPath(%q): want %q, got %q", s, expected, path)
			}

			// Compliance: executable, handles --output-dir=, writes results.jsonl.
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("stat %q: %v", path, err)
			}
			if info.Mode().Perm()&0111 == 0 {
				t.Errorf("script %q is not executable (mode %o)", path, info.Mode().Perm())
			}
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %q: %v", path, err)
			}
			script := string(content)
			if !strings.Contains(script, "--output-dir") && !strings.Contains(script, "output_dir") && !strings.Contains(script, "OUTPUT_DIR") {
				t.Errorf("script %q: must handle --output-dir= argument", path)
			}
			if !strings.Contains(script, "results.jsonl") {
				t.Errorf("script %q: must write results.jsonl", path)
			}
		})
	}

	// Unknown service should error.
	_, err = resolver.ListSuites("nonexistent")
	if err == nil {
		t.Error("ListSuites(nonexistent): expected error, got nil")
	}
	_, err = resolver.ResolveScriptPath("nonexistent", "test")
	if err == nil {
		t.Error("ResolveScriptPath(nonexistent): expected error, got nil")
	}
}

// TestAllServiceScriptsExistAndAreCompliant reads the production services.txt,
// discovers every available test suite per service via ListSuites, and verifies
// each script is compliant with the CI runner contract:
//   - Exists at the resolved path
//   - Is executable
//   - Handles --output-dir= argument
//   - Writes results.jsonl to the output directory
//
// Because it discovers suites dynamically, this test does not need modification
// as services add, remove, or rename their test suites.
func TestAllServiceScriptsExistAndAreCompliant(t *testing.T) {
	const servicesPath = "/etc/Wywy-Website-Control/services.txt"
	const reposBasePath = "/usr/local/Wywy-Website"

	data, err := os.ReadFile(servicesPath)
	if err != nil {
		t.Skipf("services.txt not found at %s: %v — skipping compliance check", servicesPath, err)
	}

	repos := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ",", 2)
		if len(parts) > 1 {
			repos[parts[0]] = parts[1]
		}
	}

	if len(repos) == 0 {
		t.Fatal("no service entries found in services.txt")
	}

	resolver := NewServiceScriptResolver(repos, reposBasePath)

	for service := range repos {
		t.Run(service, func(t *testing.T) {
			suites, err := resolver.ListSuites(service)
			if err != nil {
				t.Fatalf("ListSuites(%q): %v", service, err)
			}
			if len(suites) == 0 {
				t.Skipf("no suites found for service %q", service)
			}

			for _, suite := range suites {
				t.Run(suite, func(t *testing.T) {
					scriptPath, err := resolver.ResolveScriptPath(service, suite)
					if err != nil {
						t.Fatalf("ResolveScriptPath(%q, %q): %v", service, suite, err)
					}

					// 1. Script must exist.
					info, err := os.Stat(scriptPath)
					if err != nil {
						t.Fatalf("script not found at %q: %v", scriptPath, err)
					}

					// 2. Script must be executable.
					if info.Mode().Perm()&0111 == 0 {
						t.Errorf("script %q is not executable (mode %o)", scriptPath, info.Mode().Perm())
					}

					// 3. Script content must handle --output-dir= argument.
					content, err := os.ReadFile(scriptPath)
					if err != nil {
						t.Fatalf("read script %q: %v", scriptPath, err)
					}
					script := string(content)

					if !strings.Contains(script, "--output-dir") && !strings.Contains(script, "output_dir") && !strings.Contains(script, "OUTPUT_DIR") {
						t.Errorf("script %q: must parse --output-dir= argument (no reference to output-dir found)", scriptPath)
					}

					// 4. Script must write results.jsonl.
					if !strings.Contains(script, "results.jsonl") {
						t.Errorf("script %q: must write results.jsonl to the output directory", scriptPath)
					}
				})
			}
		})
	}
}

// TestMonitorScriptOutputReportsInvalidJSONL verifies that when a test script
// writes malformed results.jsonl (e.g. from a shell quoting bug), the Go server
// catches it and returns a non-empty parse error rather than silently accepting
// the bad output.
func TestMonitorScriptOutputReportsInvalidJSONL(t *testing.T) {
	outputDir := t.TempDir()

	// Inline a script with the exact quoting bug present in master-database's
	// test.sh: double-quoted echo produces key names without quotes.
	//   echo "{"name":"compliance","status":"passed"}"
	// The shell interprets inner " as delimiters, writing: {name:compliance,status:passed}
	scriptContent := `#!/bin/sh
output_dir=""
for arg in "$@"; do
  case "$arg" in
    --output-dir=*) output_dir="${arg#*=}" ;;
  esac
done
if [ -n "$output_dir" ]; then
  echo "{"name":"compliance","status":"passed"}" > "$output_dir/results.jsonl"
fi
`
	script := filepath.Join(t.TempDir(), "broken.sh")
	if err := os.WriteFile(script, []byte(scriptContent), 0755); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	runner := &DetachedRunner{}
	cmd, err := runner.StartDetached(ctx, outputDir, "sh", script, "--output-dir="+outputDir)
	if err != nil {
		t.Fatalf("StartDetached: %v", err)
	}
	defer cmd.Wait()
	defer cmd.Process.Signal(syscall.SIGKILL)

	_, _, _, err = testrunner.MonitorScriptOutput(ctx, outputDir, 5*time.Second)
	if err == nil {
		t.Fatal("testrunner.MonitorScriptOutput: expected error for malformed JSONL, got nil")
	}
	if err.Error() == "" {
		t.Fatal("MonitorScriptOutput: returned empty error message")
	}
	t.Logf("Got expected parse error: %v", err)
}
