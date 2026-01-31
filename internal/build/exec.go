package build

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
)

type execError struct {
	Path   string
	Code   int
	Stdout string
	Stderr string
}

func (e *execError) Error() string {
	msg := fmt.Sprintf("exit %d", e.Code)
	if e.Stderr != "" {
		msg += ": " + e.Stderr
	}
	return msg
}

func runBuild(ctx context.Context, jobDir, entrypoint string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("build: %w", err)
	}

	// Execute directly to preserve shebang and executable bit.
	path := filepath.Join(jobDir, entrypoint)
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("abs %s: %w", path, err)
	}
	cmd := exec.CommandContext(ctx, abs)
	cmd.Dir = jobDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("start %s: %w", abs, err)
	}
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case err := <-done:
		if err == nil {
			return nil
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code := exitErr.ExitCode()
			return &execError{Path: abs, Code: code, Stdout: stdout.String(), Stderr: trimOneLine(stderr.String())}
		}
		return fmt.Errorf("run %s: %w", abs, err)
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		return fmt.Errorf("build: %w", ctx.Err())
	}
}

func trimOneLine(s string) string {
	// Keep error messages stable and short; stderr is still available in execError.
	for i := range len(s) {
		if s[i] == '\n' || s[i] == '\r' {
			return s[:i]
		}
	}
	return s
}
