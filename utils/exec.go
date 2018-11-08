// +build !windows

package utils

import (
	"context"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// ResolveBinary finds a binary name along the path and evaluates any symlinks
func ResolveBinary(binname string) (string, error) {
	binaryPath, err := exec.LookPath(binname)
	if err != nil {
		return "", err
	}
	resolvedPath, err := filepath.EvalSymlinks(binaryPath)
	if err != nil {
		return "", err
	}
	return resolvedPath, nil
}

// ExecTimedCmdNoOut executes a command and returns any errors, but ignores output
// This function also times the command and returns the elapsed milliseconds
func ExecTimedCmdNoOut(ctx context.Context, cmd, args string) (string, time.Duration, error) {
	start := time.Now()
	execCmd := exec.CommandContext(ctx, cmd, strings.Split(args, " ")...)
	execCmd.Stdin = nil
	execCmd.Stdout = nil
	execCmd.Stderr = nil
	err := execCmd.Run()
	elapsed := time.Since(start)
	return "", elapsed, errors.Wrapf(err, "exec failed: %s %s", cmd, args)
}

// ExecTimedCmd executes a command and returns the combined err/out output and any errors
// This function also times the command and returns the elapsed milliseconds
func ExecTimedCmd(ctx context.Context, cmd, args string) (string, time.Duration, error) {
	start := time.Now()
	execCmd := exec.CommandContext(ctx, cmd, strings.Split(args, " ")...)
	out, err := execCmd.CombinedOutput()
	elapsed := time.Since(start)
	return string(out), elapsed, errors.Wrapf(err, "exec failed: %s %s", cmd, args)
}

// ExecCmd executes a command and returns the combined err/out output and any errors
func ExecCmd(ctx context.Context, cmd, args string) (string, error) {
	execCmd := exec.CommandContext(ctx, cmd, strings.Split(args, " ")...)
	out, err := execCmd.CombinedOutput()
	return string(out), errors.Wrapf(err, "exec failed: %s %s", cmd, args)
}

// ExecShellCmd executes a 'bash -c' process, with the passed-in command
// handed to the -c flag of bash
func ExecShellCmd(ctx context.Context, cmd string) (string, error) {
	execCmd := exec.CommandContext(ctx, "bash", "-c", cmd)
	out, err := execCmd.CombinedOutput()
	return string(out), errors.Wrapf(err, "exec failed: %s", cmd)
}

// ExecCmdStream executes a command and returns a Reader, which is useful for streaming
func ExecCmdStream(ctx context.Context, cmd, args string) (io.ReadCloser, error) {
	reader, writer := io.Pipe()

	execCmd := exec.CommandContext(ctx, cmd, strings.Split(args, " ")...)
	execCmd.Stdout = writer

	if err := execCmd.Start(); err != nil {
		return nil, err
	}

	go func() {
		err := execCmd.Wait()
		writer.CloseWithError(err)
	}()

	return reader, nil
}
