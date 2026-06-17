package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
)

// CommandRunner abstracts shell command execution for testability.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (int, string, error)
}

// realRunner executes commands via os/exec.
type realRunner struct{}

func (r *realRunner) Run(ctx context.Context, name string, args ...string) (int, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
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

// DefaultRunner is the real shell executor.
var DefaultRunner CommandRunner = &realRunner{}

// Execute runs a test suite for a service, writing output to writer.
// It returns the exit code of the test process.
func Execute(ctx context.Context, serviceName, suite string, writer io.Writer) (int, error) {
	// For now, use echo to prove plumbing works. Service resolution comes in O2.
	code, output, err := DefaultRunner.Run(ctx, "sh", "-c", fmt.Sprintf("echo 'test output for %s %s'", serviceName, suite))
	if err != nil {
		return code, err
	}
	io.WriteString(writer, output)
	return code, nil
}
