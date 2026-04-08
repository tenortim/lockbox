//go:build linux

package cache

import (
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

const keyPrefix = "lockbox:"

// KeyringCache implements SessionCache using the Linux kernel keyring.
type KeyringCache struct {
	// keyringID is the keyring to use. Typically unix.KEY_SPEC_SESSION_KEYRING
	// or unix.KEY_SPEC_USER_KEYRING.
	keyringID int
}

func NewKeyringCache() *KeyringCache {
	return &KeyringCache{keyringID: unix.KEY_SPEC_SESSION_KEYRING}
}

func NewUserKeyringCache() *KeyringCache {
	return &KeyringCache{keyringID: unix.KEY_SPEC_USER_KEYRING}
}

func (k *KeyringCache) Store(name, value string) error {
	desc := keyPrefix + name
	payload := []byte(value)
	_, err := unix.AddKey("user", desc, payload, k.keyringID)
	if err != nil {
		return fmt.Errorf("keyctl add_key: %w", err)
	}
	return nil
}

func (k *KeyringCache) Retrieve(name string) (string, error) {
	desc := keyPrefix + name
	id, err := unix.KeyctlSearch(k.keyringID, "user", desc, 0)
	if err != nil {
		return "", fmt.Errorf("secret '%s' not found in session cache", name)
	}

	// First call to get the size.
	sz, err := unix.KeyctlBuffer(unix.KEYCTL_READ, id, nil, 0)
	if err != nil {
		return "", fmt.Errorf("keyctl read size: %w", err)
	}

	buf := make([]byte, sz)
	_, err = unix.KeyctlBuffer(unix.KEYCTL_READ, id, buf, 0)
	if err != nil {
		return "", fmt.Errorf("keyctl read: %w", err)
	}

	// Copy out the value and zero the buffer.
	val := string(buf)
	for i := range buf {
		buf[i] = 0
	}
	return val, nil
}

const internalPrefix = "__"

func (k *KeyringCache) List() ([]string, error) {
	all, err := k.listRaw()
	if err != nil {
		return nil, err
	}
	// Filter out internal metadata keys (prefixed with "__").
	var names []string
	for _, name := range all {
		if !strings.HasPrefix(name, internalPrefix) {
			names = append(names, name)
		}
	}
	return names, nil
}

// listRaw returns all lockbox key names including internal metadata keys.
func (k *KeyringCache) listRaw() ([]string, error) {
	// Read the keyring contents as a packed array of key_serial_t (int32).
	sz, err := unix.KeyctlBuffer(unix.KEYCTL_READ, k.keyringID, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("keyctl read keyring: %w", err)
	}
	if sz == 0 {
		return nil, nil
	}

	buf := make([]byte, sz)
	_, err = unix.KeyctlBuffer(unix.KEYCTL_READ, k.keyringID, buf, 0)
	if err != nil {
		return nil, fmt.Errorf("keyctl read keyring: %w", err)
	}

	// Each entry is a 4-byte key serial number.
	numKeys := sz / 4
	var names []string
	for i := 0; i < numKeys; i++ {
		keyID := *(*int32)(unsafe.Pointer(&buf[i*4]))

		// Get the key description.
		descBuf := make([]byte, 256)
		n, err := unix.KeyctlBuffer(unix.KEYCTL_DESCRIBE, int(keyID), descBuf, 0)
		if err != nil {
			continue // skip keys we can't describe (may not be ours)
		}
		// Kernel may include a trailing null byte.
		desc := strings.TrimRight(string(descBuf[:n]), "\x00")

		// Description format: "type;uid;gid;perm;description"
		parts := strings.SplitN(desc, ";", 5)
		if len(parts) < 5 {
			continue
		}
		keyDesc := parts[4]
		if strings.HasPrefix(keyDesc, keyPrefix) {
			names = append(names, strings.TrimPrefix(keyDesc, keyPrefix))
		}
	}
	return names, nil
}

func (k *KeyringCache) Clear() error {
	// Clear all lockbox keys including metadata.
	// Use KEYCTL_UNLINK to remove keys from the keyring (not just REVOKE,
	// which marks them unusable but leaves them in the listing).
	names, err := k.listRaw()
	if err != nil {
		return err
	}
	for _, name := range names {
		desc := keyPrefix + name
		id, err := unix.KeyctlSearch(k.keyringID, "user", desc, 0)
		if err != nil {
			continue
		}
		unix.KeyctlInt(unix.KEYCTL_UNLINK, id, k.keyringID, 0, 0)
	}
	return nil
}

func (k *KeyringCache) IsAvailable() bool {
	// Try to get the session keyring ID; if this fails, the keyring isn't usable.
	_, err := unix.KeyctlGetKeyringID(k.keyringID, false)
	return err == nil
}
