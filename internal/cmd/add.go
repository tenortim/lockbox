package cmd

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/tenortim/lockbox/internal/store"
)

var (
	addEnvVar  string
	addDesc    string
	addExpires string
)

var addCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a secret to the store",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if addEnvVar == "" {
			return fmt.Errorf("--env is required")
		}
		if !validEnvVarName(addEnvVar) {
			return fmt.Errorf("invalid env var name %q: must match [A-Za-z_][A-Za-z0-9_]*", addEnvVar)
		}

		var expiresAt *time.Time
		if addExpires != "" {
			t, err := parseExpiry(addExpires)
			if err != nil {
				return fmt.Errorf("invalid --expires value: %w", err)
			}
			expiresAt = &t
		}

		value, err := readPassword("Enter secret value: ")
		if err != nil {
			return err
		}
		if value == "" {
			return fmt.Errorf("secret value cannot be empty")
		}

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

		secret := &store.Secret{
			Value:       value,
			EnvVar:      addEnvVar,
			Description: addDesc,
			CreatedAt:   time.Now().UTC(),
			ExpiresAt:   expiresAt,
		}
		if err := data.AddSecret(name, secret); err != nil {
			return err
		}

		if err := store.Save(storePath, pw, data); err != nil {
			return err
		}

		msg := fmt.Sprintf("Secret '%s' added (env: %s)", name, addEnvVar)
		if expiresAt != nil {
			msg += fmt.Sprintf(", expires %s", expiresAt.Format("2006-01-02"))
		}
		fmt.Fprintln(cmd.OutOrStdout(), msg)
		return nil
	},
}

var durationRe = regexp.MustCompile(`^(\d+)([dDwWmMyY])$`)
var envVarRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func validEnvVarName(name string) bool {
	return envVarRe.MatchString(name)
}

// parseExpiry parses an expiry string which can be:
//   - A date: "2024-12-31" or "2024-12-31T15:04:05Z"
//   - A duration from now: "90d" (days), "12w" (weeks), "6m" (months), "1y" (years)
func parseExpiry(s string) (time.Time, error) {
	if m := durationRe.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		now := time.Now().UTC()
		switch m[2] {
		case "d", "D":
			return now.AddDate(0, 0, n), nil
		case "w", "W":
			return now.AddDate(0, 0, n*7), nil
		case "m", "M":
			return now.AddDate(0, n, 0), nil
		case "y", "Y":
			return now.AddDate(n, 0, 0), nil
		}
	}

	// Try date formats.
	for _, layout := range []string{
		"2006-01-02",
		"2006-01-02T15:04:05Z07:00",
		time.RFC3339,
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}

	return time.Time{}, fmt.Errorf("expected a date (2024-12-31) or duration (90d, 12w, 6m, 1y)")
}

func init() {
	addCmd.Flags().StringVar(&addEnvVar, "env", "", "environment variable name (required)")
	addCmd.Flags().StringVar(&addDesc, "desc", "", "human-readable description")
	addCmd.Flags().StringVar(&addExpires, "expires", "", "expiry date (2024-12-31) or duration from now (90d, 12w, 6m, 1y)")
	rootCmd.AddCommand(addCmd)
}
