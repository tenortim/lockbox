package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tenortim/lockbox/internal/store"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a new encrypted secret store",
	RunE: func(cmd *cobra.Command, args []string) error {
		pw, err := readPassword("Enter master password: ")
		if err != nil {
			return err
		}
		if pw == "" {
			return fmt.Errorf("master password cannot be empty")
		}

		confirm, err := readPassword("Confirm master password: ")
		if err != nil {
			return err
		}
		if pw != confirm {
			return fmt.Errorf("passwords do not match")
		}

		if err := store.Create(storePath, pw); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Store created at %s\n", storePath)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
