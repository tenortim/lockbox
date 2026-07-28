package cmd

import (
	"fmt"
	"time"

	"github.com/tenortim/lockbox/internal/cache"
	"github.com/tenortim/lockbox/internal/store"
)

func sessionCacheUnlocked(c cache.SessionCache) (bool, error) {
	if !c.IsAvailable() {
		return false, nil
	}

	names, err := c.List()
	if err != nil {
		return false, fmt.Errorf("checking session cache: %w", err)
	}
	return len(names) > 0, nil
}

func refreshSessionCache(c cache.SessionCache, data *store.StoreData) error {
	if err := c.Clear(); err != nil {
		return fmt.Errorf("clearing session cache: %w", err)
	}

	for name, secret := range data.Secrets {
		if err := cacheSecret(c, name, secret); err != nil {
			if clearErr := c.Clear(); clearErr != nil {
				return fmt.Errorf("%w; clearing partial session cache: %v", err, clearErr)
			}
			return err
		}
	}
	return nil
}

func cacheSecret(c cache.SessionCache, name string, secret *store.Secret) error {
	entries := []struct {
		name  string
		value string
		label string
	}{
		{name: name, value: secret.Value, label: "secret"},
		{name: "__env__" + name, value: secret.EnvVar, label: "env mapping"},
		{name: "__desc__" + name, value: secret.Description, label: "description"},
	}
	if secret.ExpiresAt != nil {
		entries = append(entries, struct {
			name  string
			value string
			label string
		}{
			name:  "__expires__" + name,
			value: secret.ExpiresAt.Format(time.RFC3339),
			label: "expiry",
		})
	}

	for _, entry := range entries {
		if err := c.Store(entry.name, entry.value); err != nil {
			return fmt.Errorf("caching %s for '%s': %w", entry.label, name, err)
		}
	}
	return nil
}
