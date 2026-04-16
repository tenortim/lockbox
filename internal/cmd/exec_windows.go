//go:build windows

package cmd

import (
	"os"
	"os/exec"
)

// execProcess launches a child process and waits for it to exit, then exits
// lockbox with the same code. Windows has no execve(2) equivalent, so lockbox
// remains alive as the parent for the duration of the child's run. The secrets
// present in env are in lockbox's memory until the child exits; callers should
// not hold them longer than necessary.
func execProcess(binary string, args, env []string) error {
	cmd := exec.Command(binary, args[1:]...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}
	os.Exit(0)
	return nil // unreachable
}
