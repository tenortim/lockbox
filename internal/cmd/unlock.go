package cmd

import (
	"fmt"

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

		if err := refreshSessionCache(c, data); err != nil {
			return err
		}

		count := 0
		var warnings []string
		for name, secret := range data.Secrets {
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
