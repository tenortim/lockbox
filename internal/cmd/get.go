package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tenortim/lockbox/internal/store"
)

var getCmd = &cobra.Command{
	Use:               "get <name>",
	Short:             "Retrieve a secret value",
	Long:              "Retrieve a secret value from the session cache (if unlocked) or directly from the encrypted store.",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeSecretNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Try session cache first.
		c := getCache()
		if c.IsAvailable() {
			val, err := c.Retrieve(name)
			if err == nil {
				fmt.Fprint(cmd.OutOrStdout(), val)
				return nil
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

		secret, ok := data.Secrets[name]
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
