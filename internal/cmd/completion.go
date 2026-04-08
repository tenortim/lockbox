package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion <bash|zsh|fish|powershell>",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for lockbox.

To load completions:

Bash:
  eval "$(lockbox completion bash)"

  # Or persist across sessions:
  lockbox completion bash > /etc/bash_completion.d/lockbox

Zsh:
  eval "$(lockbox completion zsh)"

  # Or persist across sessions (adjust fpath as needed):
  lockbox completion zsh > "${fpath[1]}/_lockbox"

Fish:
  lockbox completion fish | source

  # Or persist across sessions:
  lockbox completion fish > ~/.config/fish/completions/lockbox.fish

PowerShell:
  lockbox completion powershell | Out-String | Invoke-Expression
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletionV2(os.Stdout, true)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}

// completeSecretNames returns a Cobra completion function that suggests
// secret names from the session cache.
func completeSecretNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	c := getCache()
	if !c.IsAvailable() {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	names, err := c.List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
