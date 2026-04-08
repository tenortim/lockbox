package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var lockCmd = &cobra.Command{
	Use:   "lock",
	Short: "Clear all secrets from the session cache",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := getCache()
		if !c.IsAvailable() {
			fmt.Fprintln(cmd.OutOrStdout(), "Session cache not available; nothing to clear.")
			return nil
		}

		if err := c.Clear(); err != nil {
			return fmt.Errorf("clearing session cache: %w", err)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Session cache cleared.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(lockCmd)
}
