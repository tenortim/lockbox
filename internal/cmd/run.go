package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tenortim/lockbox/internal/store"
)

var runSecrets string

var runCmd = &cobra.Command{
	Use:   "run [--secrets name,...] -- cmd [args...]",
	Short: "Run a command with secrets injected as environment variables",
	Long: `Run a command with secrets injected as ephemeral environment variables.
The secrets only exist in the child process's environment, never in the
parent shell. If the session cache is unlocked, secrets are read from there;
otherwise, the store is decrypted on-the-fly.`,
	DisableFlagParsing: false,
	Args:               cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		secrets, err := resolveSecrets(runSecrets)
		if err != nil {
			return err
		}

		env := os.Environ()
		for _, s := range secrets {
			env = append(env, s.EnvVar+"="+s.Value)
		}

		// Use exec to replace the process if possible.
		binary, err := exec.LookPath(args[0])
		if err != nil {
			return fmt.Errorf("command not found: %s", args[0])
		}

		return execProcess(binary, args, env)
	},
}

type resolvedSecret struct {
	Name   string
	Value  string
	EnvVar string
}

func resolveSecrets(filter string) ([]resolvedSecret, error) {
	var filterSet map[string]bool
	if filter != "" {
		filterSet = make(map[string]bool)
		for _, name := range strings.Split(filter, ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				filterSet[name] = true
			}
		}
	}

	// Try session cache first.
	c := getCache()
	if c.IsAvailable() {
		names, err := c.List()
		if err == nil && len(names) > 0 {
			return resolveFromCache(c, names, filterSet)
		}
	}

	// Fall back to decrypting the store.
	return resolveFromStore(filterSet)
}

func resolveFromCache(c cacheWithStore, names []string, filterSet map[string]bool) ([]resolvedSecret, error) {
	// We need the env_var mapping from the store, but can get values from cache.
	// To avoid requiring the password just for mappings, we also cache the env_var
	// mapping in the keyring with a special prefix.
	var result []resolvedSecret
	for _, name := range names {
		if filterSet != nil && !filterSet[name] {
			continue
		}
		val, err := c.Retrieve(name)
		if err != nil {
			continue
		}
		envVar, err := c.Retrieve("__env__" + name)
		if err != nil {
			continue
		}
		result = append(result, resolvedSecret{Name: name, Value: val, EnvVar: envVar})
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no secrets found in session cache")
	}
	return result, nil
}

type cacheWithStore interface {
	Retrieve(name string) (string, error)
}

func resolveFromStore(filterSet map[string]bool) ([]resolvedSecret, error) {
	pw, err := readPassword("Enter master password: ")
	if err != nil {
		return nil, err
	}

	data, err := store.Open(storePath, pw)
	if err != nil {
		return nil, err
	}

	var result []resolvedSecret
	for name, secret := range data.Secrets {
		if filterSet != nil && !filterSet[name] {
			continue
		}
		result = append(result, resolvedSecret{Name: name, Value: secret.Value, EnvVar: secret.EnvVar})
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no matching secrets found")
	}
	return result, nil
}

func init() {
	runCmd.Flags().StringVar(&runSecrets, "secrets", "", "comma-separated list of secret names to inject (default: all)")
	runCmd.RegisterFlagCompletionFunc("secrets", completeSecretNames)
	rootCmd.AddCommand(runCmd)
}
