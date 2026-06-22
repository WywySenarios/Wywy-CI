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
	inv := ScriptInvocation{
		RunID:      "run-abc",
		OutputDir:  "/tmp/runs/run-abc",
		Machine:    true,
		ExtraFlags: []string{"--verbose"},
	}

	args := BuildScriptArgs(inv)

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

	results, stdout, stderr, err := MonitorScriptOutput(ctx, outputDir, 5*time.Second)
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

	_, _, _, err := MonitorScriptOutput(ctx, outputDir, 50*time.Millisecond)
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

// TestAllServiceScriptsExistAndAreCompliant reads the production services.txt,
// resolves each service's "test" suite script, and verifies every script is
// compliant with the CI runner contract:
//   - Exists at the resolved path
//   - Is executable
//   - Handles --output-dir= argument
//   - Writes results.jsonl to the output directory
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
			scriptPath, err := resolver.ResolveScriptPath(service, "test")
			if err != nil {
				t.Fatalf("ResolveScriptPath(%q, \"test\"): %v", service, err)
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
}
