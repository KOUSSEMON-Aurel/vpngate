package exec

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
)

// tailLines is the number of trailing output lines kept for failure
// reporting.
const tailLines = 8

// Run executes a command in workDir and logs its output.
// If the command fails to start or setup fails, an error is logged and returned.
// If the command exits with a non-zero status, the error is returned without logging
// (this allows the caller to decide how to handle it).
func Run(path string, workDir string, args ...string) error {
	_, err := exec.LookPath(path)
	if err != nil {
		log.Error().Msgf("%s is required, please install it", path)
		return err
	}

	cmd := exec.Command(path, args...)
	cmd.Dir = workDir

	log.Debug().Strs("command", cmd.Args).Msg("Executing command")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Error().Msgf("Failed to get stdout pipe: %v", err)
		return err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Error().Msgf("Failed to get stderr pipe: %v", err)
		return err
	}

	if err := cmd.Start(); err != nil {
		log.Error().Msgf("Failed to start command: %v", err)
		return err
	}

	// stdout and stderr must be drained concurrently: reading them
	// sequentially (e.g. via io.MultiReader) can deadlock the child if it
	// fills the unread pipe's OS buffer before the read one reaches EOF.
	var wg sync.WaitGroup
	var stdoutErr, stderrErr error
	var stdoutTail, stderrTail []string

	wg.Add(2)
	go func() {
		defer wg.Done()
		stdoutErr = logLines(stdout, &stdoutTail)
	}()
	go func() {
		defer wg.Done()
		stderrErr = logLines(stderr, &stderrTail)
	}()
	wg.Wait()

	if stdoutErr != nil {
		log.Error().Msgf("Error reading stdout: %v", stdoutErr)
		return stdoutErr
	}
	if stderrErr != nil {
		log.Error().Msgf("Error reading stderr: %v", stderrErr)
		return stderrErr
	}

	// cmd.Wait() returns an error if the command exits with non-zero status.
	// We return this without logging since it's expected behavior for some
	// commands, but append the tail of the command's output so callers can
	// surface the underlying reason.
	if err := cmd.Wait(); err != nil {
		if msg := tailMessage(stdoutTail, stderrTail); msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}

// logLines streams lines from r through a debug log, keeping a capped
// ring of the most recent lines for failure reporting.
func logLines(r io.Reader, tail *[]string) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		log.Debug().Msg(line)
		*tail = append(*tail, line)
		if len(*tail) > tailLines {
			*tail = (*tail)[1:]
		}
	}
	return scanner.Err()
}

// tailMessage joins the trailing output of a failed command into a single
// line, or returns "" when there is nothing useful to report.
func tailMessage(stdoutTail, stderrTail []string) string {
	lines := append(append([]string{}, stdoutTail...), stderrTail...)
	clean := make([]string, 0, len(lines))
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			clean = append(clean, l)
		}
	}
	if len(clean) > tailLines {
		clean = clean[len(clean)-tailLines:]
	}
	return strings.Join(clean, " | ")
}
