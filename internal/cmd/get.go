package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tenortim/lockbox/internal/store"
)

var getCmd = &cobra.Command{
	Use:               "get <name>",
	Short:             "Retrieve a secret value",
	Long:              "Retrieve a secret value from the session cache (if unlocked) or directly from the encrypted store. Accepts either the entry name or the environment variable name.",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeSecretNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Try session cache first.
		c := getCache()
		if c.IsAvailable() {
			names, err := c.List()
			if err == nil && len(names) > 0 {
				// Direct entry name match.
				if val, err := c.Retrieve(name); err == nil {
					fmt.Fprint(cmd.OutOrStdout(), val)
					return nil
				}
				// Env var name match.
				for _, entryName := range names {
					if envVar, err := c.Retrieve("__env__" + entryName); err == nil && envVar == name {
						if val, err := c.Retrieve(entryName); err == nil {
							fmt.Fprint(cmd.OutOrStdout(), val)
							return nil
						}
					}
				}
				return fmt.Errorf("secret '%s' not found", name)
			}
		}

		// Fall back to decrypting the store.
		pw, err := readPassword("Enter master password: ")
		if err != nil {
			return err
		}

		data, err := store.Open(storePath, pw)
		if err != nil {
			return err
		}

		// Direct entry name match.
		secret, ok := data.Secrets[name]
		if !ok {
			// Env var name match.
			for _, s := range data.Secrets {
				if s.EnvVar == name {
					secret = s
					ok = true
					break
				}
			}
		}
		if !ok {
			return fmt.Errorf("secret '%s' not found", name)
		}

		fmt.Fprint(cmd.OutOrStdout(), secret.Value)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(getCmd)
}
