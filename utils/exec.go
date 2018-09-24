// +build !windows

package utils

import (
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
func ExecTimedCmdNoOut(cmd, args string) (string, int, error) {
	start := time.Now()
	execCmd := exec.Command(cmd, strings.Split(args, " ")...)
	execCmd.Stdin = nil
	execCmd.Stdout = nil
	execCmd.Stderr = nil
	err := execCmd.Run()
	elapsed := time.Since(start)
	msElapsed := int(elapsed.Nanoseconds() / 1000000)
	return "", msElapsed, errors.Wrapf(err, "exec failed: %s %s", cmd, args)
}

// ExecTimedCmd executes a command and returns the combined err/out output and any errors
// This function also times the command and returns the elapsed milliseconds
func ExecTimedCmd(cmd, args string) (string, int, error) {
	start := time.Now()
	execCmd := exec.Command(cmd, strings.Split(args, " ")...)
	out, err := execCmd.CombinedOutput()
	elapsed := time.Since(start)
	msElapsed := int(elapsed.Nanoseconds() / 1000000)
	return string(out), msElapsed, errors.Wrapf(err, "exec failed: %s %s", cmd, args)
}

// ExecCmd executes a command and returns the combined err/out output and any errors
func ExecCmd(cmd, args string) (string, error) {
	execCmd := exec.Command(cmd, strings.Split(args, " ")...)
	out, err := execCmd.CombinedOutput()
	return string(out), errors.Wrapf(err, "exec failed: %s %s", cmd, args)
}

// ExecShellCmd executes a 'bash -c' process, with the passed-in command
// handed to the -c flag of bash
func ExecShellCmd(cmd string) (string, error) {
	execCmd := exec.Command("bash", "-c", cmd)
	out, err := execCmd.CombinedOutput()
	return string(out), errors.Wrapf(err, "exec failed: %s", cmd)
}
