package subprocess

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// DefaultTimeout is the default subprocess execution timeout.
const DefaultTimeout = 30 * time.Second

// Runner executes an external binary with stdin/stdout pipes.
type Runner struct {
	BinaryPath string
	Timeout    time.Duration
}

// Result holds the output of a subprocess invocation.
type Result struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// Run executes the binary with args, writing stdin to the process and collecting output.
func (r *Runner) Run(ctx context.Context, args []string, stdin io.Reader) (*Result, error) {
	timeout := r.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, r.BinaryPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if stdin != nil {
		cmd.Stdin = stdin
	}

	err := cmd.Run()
	res := &Result{
		Stdout: stdout.Bytes(),
		Stderr: stderr.Bytes(),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			res.ExitCode = exitErr.ExitCode()
			return res, fmt.Errorf("subprocess exited %d: %s", res.ExitCode, stderr.String())
		}
		if ctx.Err() == context.DeadlineExceeded {
			return res, fmt.Errorf("subprocess timed out after %s", timeout)
		}
		return res, fmt.Errorf("subprocess error: %w", err)
	}
	return res, nil
}
