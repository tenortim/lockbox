package cmd

import (
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/tenortim/lockbox/internal/store"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List secrets in the store",
	RunE: func(cmd *cobra.Command, args []string) error {
		pw, err := readPassword("Enter master password: ")
		if err != nil {
			return err
		}

		data, err := store.Open(storePath, pw)
		if err != nil {
			return err
		}

		if len(data.Secrets) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No secrets in store.")
			return nil
		}

		names := make([]string, 0, len(data.Secrets))
		for name := range data.Secrets {
			names = append(names, name)
		}
		sort.Strings(names)

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tENV VAR\tEXPIRES\tDESCRIPTION")
		for _, name := range names {
			s := data.Secrets[name]
			expiry := s.ExpiryStatus()
			if expiry == "" {
				expiry = "-"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, s.EnvVar, expiry, s.Description)
		}
		return w.Flush()
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
