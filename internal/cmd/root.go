package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/tenortim/lockbox/internal/cache"
	"github.com/tenortim/lockbox/internal/store"
)

var storePath string

var rootCmd = &cobra.Command{
	Use:   "lockbox",
	Short: "Secure secret management for headless systems",
	Long: `lockbox provides ssh-agent-style secret management for headless systems.
Secrets are persisted in an age-encrypted store and cached in a platform session
store (Linux kernel keyring, Windows Credential Manager) during a session. Use
'lockbox run' to inject secrets as ephemeral environment variables into child
processes.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&storePath, "store", store.DefaultStorePath(), "path to encrypted store file")
}

func getCache() cache.SessionCache {
	return cache.NewUserKeyringCache()
}

// readPassword reads a password from the terminal, printing an asterisk for
// each character typed and handling backspace. This gives visual feedback that
// helps catch double-pastes and other input errors.
func readPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)

	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return "", fmt.Errorf("setting terminal raw mode: %w", err)
	}
	defer term.Restore(fd, oldState)

	var pw []byte
	var buf [1]byte
	var escRemain int // bytes remaining in an escape sequence to skip
	for {
		_, err := os.Stdin.Read(buf[:])
		if err != nil {
			fmt.Fprint(os.Stderr, "\r\n")
			store.ZeroBytes(pw)
			return "", fmt.Errorf("reading input: %w", err)
		}
		b := buf[0]

		// Consume remaining bytes of an escape sequence (e.g. arrow keys).
		if escRemain > 0 {
			escRemain--
			continue
		}

		switch {
		case b == '\r' || b == '\n':
			fmt.Fprint(os.Stderr, "\r\n")
			result := string(pw)
			store.ZeroBytes(pw)
			return result, nil
		case b == 3: // Ctrl+C
			fmt.Fprint(os.Stderr, "\r\n")
			store.ZeroBytes(pw)
			return "", fmt.Errorf("interrupted")
		case b == 127 || b == 8: // DEL or BS
			if len(pw) > 0 {
				pw = pw[:len(pw)-1]
				fmt.Fprint(os.Stderr, "\b \b")
			}
		case b == 27: // ESC - start of escape sequence
			escRemain = 2
		case b >= 32: // printable character
			pw = append(pw, b)
			fmt.Fprint(os.Stderr, "*")
		}
	}
}
