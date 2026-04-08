package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tenortim/lockbox/internal/store"
)

var removeCmd = &cobra.Command{
	Use:               "remove <name>",
	Aliases:           []string{"rm"},
	Short:             "Remove a secret from the store",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeSecretNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		pw, err := readPassword("Enter master password: ")
		if err != nil {
			return err
		}

		unlock, err := store.Lock(storePath)
		if err != nil {
			return err
		}
		defer unlock()

		data, err := store.Open(storePath, pw)
		if err != nil {
			return err
		}

		if err := data.RemoveSecret(name); err != nil {
			return err
		}

		if err := store.Save(storePath, pw, data); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Secret '%s' removed.\n", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
}
