package exec

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestRunSurfacesOutputTailOnFailure verifies that a non-zero exit from the
// child returns an error containing the trailing lines of its output.
func TestRunSurfacesOutputTailOnFailure(t *testing.T) {
	err := Run("sh", ".", "-c", "echo first; echo boom; exit 3")
	assert.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, "exit status 3")
	assert.Contains(t, msg, "first")
	assert.Contains(t, msg, "boom")
}

// TestRunSuccessWithoutOutput verifies a clean exit returns nil even with
// no output produced.
func TestRunSuccessWithoutOutput(t *testing.T) {
	err := Run("sh", ".", "-c", "exit 0")
	assert.NoError(t, err)
}

// TestRunTailCapped verifies only the trailing lines are kept.
func TestRunTailCapped(t *testing.T) {
	err := Run("sh", ".", "-c", "seq 1 100; exit 1")
	assert.Error(t, err)
	msg := err.Error()
	assert.False(t, strings.Contains(msg, "| 1 |"), "expected old lines to be dropped: %s", msg)
	assert.True(t, strings.Contains(msg, "100"), "expected the final line to be kept: %s", msg)
}

// TestRunContextCancelReturns verifies that canceling the context tears down
// the child and its descendants (a lingering grandchild holding the output
// pipes open must not keep RunContext blocked forever) and returns promptly.
func TestRunContextCancelReturns(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- RunContext(ctx, "sh", ".", nil, "-c", "echo hi; sleep 30; echo bye")
	}()

	time.Sleep(300 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.Error(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("RunContext did not return after context cancel")
	}
}

// TestRunContextStreamsLines verifies lines produced by the child are
// delivered to the output writer.
func TestRunContextStreamsLines(t *testing.T) {
	var got strings.Builder
	err := RunContext(context.Background(), "sh", ".", &got, "-c", "echo one; echo two")
	assert.NoError(t, err)
	assert.Contains(t, got.String(), "one\n")
	assert.Contains(t, got.String(), "two\n")
}
