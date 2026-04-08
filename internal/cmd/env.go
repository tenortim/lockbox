package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var envSecrets string

var envCmd = &cobra.Command{
	Use:   "env [--secrets name,...]",
	Short: "Print export statements for eval",
	Long: `Print 'export KEY=VALUE' statements suitable for eval:

  eval $(lockbox env)

This exports secrets into the current shell's environment. Less secure than
'lockbox run' since the secrets persist in the shell environment and are
visible to all subsequent child processes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		secrets, err := resolveSecrets(envSecrets)
		if err != nil {
			return err
		}

		for _, s := range secrets {
			fmt.Fprintf(cmd.OutOrStdout(), "export %s=%q\n", s.EnvVar, s.Value)
		}
		return nil
	},
}

func init() {
	envCmd.Flags().StringVar(&envSecrets, "secrets", "", "comma-separated list of secret names to export (default: all)")
	envCmd.RegisterFlagCompletionFunc("secrets", completeSecretNames)
	rootCmd.AddCommand(envCmd)
}
