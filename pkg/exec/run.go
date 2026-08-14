package exec

import (
	"bufio"
	"context"
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
	return run(context.Background(), path, workDir, nil, args...)
}

// RunContext is Run with a cancelable context and an optional output
// stream: when ctx is canceled the child process is killed, and when out
// is non-nil each output line is written to it (followed by a newline) as
// it is produced.
func RunContext(ctx context.Context, path string, workDir string, out io.Writer, args ...string) error {
	return run(ctx, path, workDir, out, args...)
}

func run(ctx context.Context, path string, workDir string, out io.Writer, args ...string) error {
	_, err := exec.LookPath(path)
	if err != nil {
		log.Error().Msgf("%s is required, please install it", path)
		return err
	}

	cmd := exec.Command(path, args...)
	cmd.Dir = workDir
	prepareProcGroup(cmd)

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

	// Kill the child's process group when the context is canceled.
	finished := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			killProcGroup(cmd)
		case <-finished:
		}
	}()

	// stdout and stderr must be drained concurrently: reading them
	// sequentially (e.g. via io.MultiReader) can deadlock the child if it
	// fills the unread pipe's OS buffer before the read one reaches EOF.
	var wg sync.WaitGroup
	var stdoutErr, stderrErr error
	var stdoutTail, stderrTail []string

	wg.Add(2)
	go func() {
		defer wg.Done()
		stdoutErr = streamLines(stdout, out, &stdoutTail)
	}()
	go func() {
		defer wg.Done()
		stderrErr = streamLines(stderr, out, &stderrTail)
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
	waitErr := cmd.Wait()
	close(finished)
	if waitErr != nil {
		if msg := tailMessage(stdoutTail, stderrTail); msg != "" {
			return fmt.Errorf("%w: %s", waitErr, msg)
		}
		return waitErr
	}
	return nil
}

// streamLines streams lines from r through a debug log and, when out is
// non-nil, through out, keeping a capped ring of the most recent lines for
// failure reporting.
func streamLines(r io.Reader, out io.Writer, tail *[]string) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		log.Debug().Msg(line)
		if out != nil {
			_, _ = io.WriteString(out, line+"\n")
		}
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
