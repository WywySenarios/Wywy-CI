package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"wywy-ci/apps/testrunner"
)

// ServiceScriptResolver resolves script paths from service names.
// It is an alias for testrunner.ServiceScriptResolver.
type ServiceScriptResolver = testrunner.ServiceScriptResolver

// NewServiceScriptResolver creates a resolver with a service→repo mapping.
var NewServiceScriptResolver = testrunner.NewServiceScriptResolver

// CommandRunner abstracts shell command execution for testability.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (int, string, error)
}

// runCmd executes the command synchronously and returns (exitCode, combinedOutput, error).
// The caller provides a fully configured *exec.Cmd (name, args, SysProcAttr, etc.).
func runCmd(cmd *exec.Cmd) (int, string, error) {
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), stdout.String(), nil
		}
		return -1, stdout.String(), fmt.Errorf("command failed: %w", err)
	}
	return 0, stdout.String(), nil
}

// DefaultRunner is the default command runner used by Execute and Runner.
// It supports both synchronous (Run) and detached (StartDetached) execution.
var DefaultRunner CommandRunner = &DetachedRunner{}

// DefaultResolver is the default ServiceScriptResolver used by Execute
// to resolve service+suite pairs to test script paths. Execute returns
// an error if DefaultResolver is nil.
var DefaultResolver *ServiceScriptResolver

// DetachedCommandRunner is an optional interface for spawn-and-forget execution.
// StartDetached starts the command in a new process group. The command's stdout
// and stderr are redirected to stdout.log and stderr.log inside outputDir.
type DetachedCommandRunner interface {
	StartDetached(ctx context.Context, outputDir, name string, args ...string) (*exec.Cmd, error)
}

// DetachedRunner spawns commands in a new process group and returns immediately.
type DetachedRunner struct{}

// newCommand creates an *exec.Cmd isolated in its own process group.
func (r *DetachedRunner) newCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}

// StartDetached starts the command in a new process group without waiting for it to finish.
// The command's stdout and stderr are redirected to stdout.log and stderr.log inside outputDir.
func (r *DetachedRunner) StartDetached(ctx context.Context, outputDir, name string, args ...string) (*exec.Cmd, error) {
	cmd := r.newCommand(ctx, name, args...)

	stdoutPath := filepath.Join(outputDir, "stdout.log")
	stderrPath := filepath.Join(outputDir, "stderr.log")

	stdoutFile, err := os.Create(stdoutPath)
	if err != nil {
		return nil, fmt.Errorf("create stdout.log: %w", err)
	}
	defer stdoutFile.Close()
	cmd.Stdout = stdoutFile

	stderrFile, err := os.Create(stderrPath)
	if err != nil {
		return nil, fmt.Errorf("create stderr.log: %w", err)
	}
	defer stderrFile.Close()
	cmd.Stderr = stderrFile

	if err := cmd.Start(); err != nil {
		return cmd, err
	}
	return cmd, nil
}

// Run executes the command synchronously in a new process group (satisfies CommandRunner).
func (r *DetachedRunner) Run(ctx context.Context, name string, args ...string) (int, string, error) {
	return runCmd(r.newCommand(ctx, name, args...))
}

// Execute runs a test suite for a service, writing output to writer.
// It resolves the test script using DefaultResolver and runs it via DefaultRunner.
// It returns the exit code of the test process.
func Execute(ctx context.Context, serviceName, suite string, writer io.Writer) (int, error) {
	if DefaultResolver == nil {
		return -1, fmt.Errorf("Execute requires DefaultResolver to be set")
	}
	scriptPath, err := DefaultResolver.ResolveScriptPath(serviceName, suite)
	if err != nil {
		return -1, err
	}
	code, output, err := DefaultRunner.Run(ctx, "sh", scriptPath)
	if err != nil {
		return code, err
	}
	io.WriteString(writer, output)
	return code, nil
}
