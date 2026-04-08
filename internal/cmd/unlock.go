package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/tenortim/lockbox/internal/store"
)

var unlockCmd = &cobra.Command{
	Use:   "unlock",
	Short: "Decrypt store and load secrets into session cache",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := getCache()
		if !c.IsAvailable() {
			return fmt.Errorf("session cache not available (kernel keyring not accessible)")
		}

		pw, err := readPassword("Enter master password: ")
		if err != nil {
			return err
		}

		data, err := store.Open(storePath, pw)
		if err != nil {
			return err
		}

		count := 0
		var warnings []string
		for name, secret := range data.Secrets {
			if err := c.Store(name, secret.Value); err != nil {
				return fmt.Errorf("caching secret '%s': %w", name, err)
			}
			// Also cache the env_var mapping so run/env can work without the store password.
			if err := c.Store("__env__"+name, secret.EnvVar); err != nil {
				return fmt.Errorf("caching env mapping for '%s': %w", name, err)
			}
			if secret.ExpiresAt != nil {
				if err := c.Store("__expires__"+name, secret.ExpiresAt.Format(time.RFC3339)); err != nil {
					return fmt.Errorf("caching expiry for '%s': %w", name, err)
				}
			}
			count++
			if secret.IsExpired() {
				warnings = append(warnings, fmt.Sprintf("  WARNING: '%s' (%s) has EXPIRED", name, secret.EnvVar))
			} else if secret.IsExpiringSoon() {
				warnings = append(warnings, fmt.Sprintf("  WARNING: '%s' (%s) %s", name, secret.EnvVar, secret.ExpiryStatus()))
			}
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%d secret(s) loaded into session cache.\n", count)
		for _, w := range warnings {
			fmt.Fprintln(cmd.ErrOrStderr(), w)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(unlockCmd)
}
