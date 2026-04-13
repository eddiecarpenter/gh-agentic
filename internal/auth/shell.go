package auth

import "os/exec"

// RunCommandFunc runs a shell command and returns its combined stdout+stderr output.
type RunCommandFunc func(name string, args ...string) (string, error)

// DefaultRunCommand is the production implementation of RunCommandFunc.
func DefaultRunCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...) //nolint:gosec
	out, err := cmd.CombinedOutput()
	return string(out), err
}
