package orchestrator

import (
	"bytes"
	"context"
	"testing"
)

func TestExecuteSingleService(t *testing.T) {
	var buf bytes.Buffer
	ctx := context.Background()

	code, err := Execute(ctx, "agentic", "test", &buf)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code: want 0, got %d", code)
	}

	output := buf.String()
	if output == "" {
		t.Error("expected output, got empty")
	}
}
