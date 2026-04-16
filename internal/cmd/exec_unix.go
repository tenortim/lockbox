//go:build unix

package cmd

import "syscall"

// execProcess replaces the current process with the given binary via execve(2).
// On success this call never returns; the calling lockbox process ceases to
// exist, so secrets held in env are only ever present in the child.
func execProcess(binary string, args, env []string) error {
	return syscall.Exec(binary, args, env)
}
