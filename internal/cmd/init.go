package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/tenortim/lockbox/internal/store"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a new encrypted secret store",
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")

		if _, err := os.Stat(storePath); err == nil {
			if !force {
				return fmt.Errorf("store already exists at %s (use --force to overwrite)", storePath)
			}
			if err := os.Remove(storePath); err != nil {
				return fmt.Errorf("removing existing store: %w", err)
			}
		}

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
	initCmd.Flags().Bool("force", false, "overwrite existing store")
	rootCmd.AddCommand(initCmd)
}
