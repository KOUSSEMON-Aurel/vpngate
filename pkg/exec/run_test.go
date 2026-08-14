package exec

import (
	"strings"
	"testing"

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
