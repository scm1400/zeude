//go:build !windows

// Package executil provides platform-specific process execution.
package executil

import (
	"syscall"
)

// Exec replaces the current process with the named program.
// On Unix, this uses syscall.Exec (never returns on success).
// On Windows, this spawns a child process and exits with its code.
func Exec(path string, args []string, env []string) error {
	return syscall.Exec(path, args, env)
}
