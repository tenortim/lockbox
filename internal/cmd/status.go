package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/tenortim/lockbox/internal/store"
)

var statusShort bool

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show whether the session is unlocked and other status info",
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		c := getCache()

		if statusShort {
			return statusShortOutput(out, c)
		}

		// Store file.
		fmt.Fprintf(out, "Store:   %s", storePath)
		if _, err := os.Stat(storePath); err == nil {
			fmt.Fprintln(out, " (exists)")
		} else {
			fmt.Fprintln(out, " (not found)")
		}

		// Session cache.
		if !c.IsAvailable() {
			fmt.Fprintln(out, "Cache:   unavailable (kernel keyring not accessible)")
			fmt.Fprintln(out, "Status:  locked")
			return nil
		}

		names, err := c.List()
		if err != nil {
			fmt.Fprintf(out, "Cache:   error (%v)\n", err)
			fmt.Fprintln(out, "Status:  unknown")
			return nil
		}

		if len(names) == 0 {
			fmt.Fprintln(out, "Cache:   empty")
			fmt.Fprintln(out, "Status:  locked")
			return nil
		}

		fmt.Fprintf(out, "Cache:   %d secret(s)\n", len(names))
		fmt.Fprintln(out, "Status:  unlocked")
		fmt.Fprintln(out)
		for _, name := range names {
			envVar, _ := c.Retrieve("__env__" + name)
			label := name
			if envVar != "" {
				label = name + " -> " + envVar
			}

			expiryStr, err := c.Retrieve("__expires__" + name)
			if err == nil {
				if t, err := time.Parse(time.RFC3339, expiryStr); err == nil {
					s := &store.Secret{ExpiresAt: &t}
					status := s.ExpiryStatus()
					if s.IsExpired() || s.IsExpiringSoon() {
						label += "  [" + status + "]"
					} else {
						label += "  (expires " + status + ")"
					}
				}
			}
			fmt.Fprintf(out, "  %s\n", label)
		}
		return nil
	},
}

func statusShortOutput(out interface{ Write([]byte) (int, error) }, c interface {
	IsAvailable() bool
	List() ([]string, error)
}) error {
	if !c.IsAvailable() {
		fmt.Fprintln(out, "locked")
		return nil
	}
	names, err := c.List()
	if err != nil || len(names) == 0 {
		fmt.Fprintln(out, "locked")
		return nil
	}
	fmt.Fprintf(out, "unlocked %d\n", len(names))
	return nil
}

func init() {
	statusCmd.Flags().BoolVar(&statusShort, "short", false, "single-line output for shell prompt integration")
	rootCmd.AddCommand(statusCmd)
}
