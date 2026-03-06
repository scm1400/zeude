//go:build windows

// Package executil provides platform-specific process execution.
package executil

import (
	"os"
	"os/exec"
	"os/signal"
)

// Exec spawns the named program as a child process with inherited stdio.
// On Windows, syscall.Exec is not available, so we use os/exec instead.
// This function blocks until the child exits, then calls os.Exit with
// the child's exit code.
func Exec(path string, args []string, env []string) error {
	cmd := exec.Command(path, args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env

	// Let Ctrl+C propagate naturally to the child process
	signal.Ignore(os.Interrupt)

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}
	os.Exit(0)
	return nil // unreachable
}
