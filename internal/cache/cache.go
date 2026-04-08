package cache

// SessionCache is the interface for platform-specific session secret caching.
// On Linux this is backed by the kernel keyring. Future implementations may
// use macOS Keychain or Windows Credential Manager.
type SessionCache interface {
	// Store adds or updates a named secret in the session cache.
	Store(name, value string) error

	// Retrieve returns the value of a named secret from the session cache.
	Retrieve(name string) (string, error)

	// List returns the names of all secrets currently in the session cache.
	List() ([]string, error)

	// Clear removes all lockbox secrets from the session cache.
	Clear() error

	// IsAvailable reports whether the session cache backend is functional.
	IsAvailable() bool
}
