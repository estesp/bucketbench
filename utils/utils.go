// +build !windows

package utils

import (
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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

// ExecTimedCmd executes a command and returns the combined err/out output and any errors
// This function also times the command and returns the elapsed milliseconds
func ExecTimedCmd(cmd, args string) (string, int, error) {
	start := time.Now()
	execCmd := exec.Command(cmd, strings.Split(args, " ")...)
	elapsed := time.Since(start)
	msElapsed := int(elapsed.Nanoseconds() / 1000000)
	out, err := execCmd.CombinedOutput()
	return string(out), msElapsed, err
}

// ExecCmd executes a command and returns the combined err/out output and any errors
func ExecCmd(cmd, args string) (string, error) {
	execCmd := exec.Command(cmd, strings.Split(args, " ")...)
	out, err := execCmd.CombinedOutput()
	return string(out), err
}
