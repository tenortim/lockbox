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

// keyPerms sets possessor and user masks to all permissions. The kernel
// default only grants read/write to the possessor, which requires the key
// to be reachable through the session keyring chain. In tmux/screen the
// session keyring is revoked, so we need read (and write for updates) in
// the user mask (UID match) as well.
//
// Security trade-off: KEY_USR_ALL means any process running as the same UID
// can read these keys, not just processes descended from the unlocking shell.
// This is acceptable for a single-user secret cache but differs from the
// tighter isolation of the session keyring.
const keyPerms = 0x3f3f0000 // KEY_POS_ALL | KEY_USR_ALL

func (k *KeyringCache) Store(name, value string) error {
	desc := keyPrefix + name
	payload := []byte(value)
	id, err := unix.AddKey("user", desc, payload, k.keyringID)
	if err != nil {
		return fmt.Errorf("keyctl add_key: %w", err)
	}
	if _, err := unix.KeyctlInt(unix.KEYCTL_SETPERM, int(id), keyPerms, 0, 0); err != nil {
		return fmt.Errorf("keyctl setperm: %w", err)
	}
	return nil
}

func (k *KeyringCache) Retrieve(name string) (string, error) {
	id, err := k.findKeyID(name)
	if err != nil {
		return "", fmt.Errorf("secret '%s' not found in session cache", name)
	}
	return k.readKey(id)
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
	entries, err := k.listEntries()
	if err != nil {
		return nil, err
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.name
	}
	return names, nil
}

type keyEntry struct {
	id   int
	name string
}

// listEntries enumerates all lockbox keys in the keyring, returning their
// serial IDs and names (including internal metadata keys).
func (k *KeyringCache) listEntries() ([]keyEntry, error) {
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
	var entries []keyEntry
	for i := 0; i < numKeys; i++ {
		keyID := *(*int32)(unsafe.Pointer(&buf[i*4]))

		descBuf := make([]byte, 256)
		n, err := unix.KeyctlBuffer(unix.KEYCTL_DESCRIBE, int(keyID), descBuf, 0)
		if err != nil {
			continue
		}
		desc := strings.TrimRight(string(descBuf[:n]), "\x00")

		// Description format: "type;uid;gid;perm;description"
		parts := strings.SplitN(desc, ";", 5)
		if len(parts) < 5 {
			continue
		}
		keyDesc := parts[4]
		if strings.HasPrefix(keyDesc, keyPrefix) {
			entries = append(entries, keyEntry{
				id:   int(keyID),
				name: strings.TrimPrefix(keyDesc, keyPrefix),
			})
		}
	}
	return entries, nil
}

// findKeyID locates a lockbox key by name and returns its serial ID.
func (k *KeyringCache) findKeyID(name string) (int, error) {
	entries, err := k.listEntries()
	if err != nil {
		return 0, err
	}
	for _, e := range entries {
		if e.name == name {
			return e.id, nil
		}
	}
	return 0, fmt.Errorf("key not found: %s", name)
}

// readKey reads the payload of a key by its serial ID, zeroing the
// intermediate buffer after copying.
func (k *KeyringCache) readKey(id int) (string, error) {
	sz, err := unix.KeyctlBuffer(unix.KEYCTL_READ, id, nil, 0)
	if err != nil {
		return "", fmt.Errorf("keyctl read size: %w", err)
	}

	buf := make([]byte, sz)
	_, err = unix.KeyctlBuffer(unix.KEYCTL_READ, id, buf, 0)
	if err != nil {
		return "", fmt.Errorf("keyctl read: %w", err)
	}

	val := string(buf)
	for i := range buf {
		buf[i] = 0
	}
	return val, nil
}

func (k *KeyringCache) Clear() error {
	entries, err := k.listEntries()
	if err != nil {
		return err
	}
	for _, e := range entries {
		unix.KeyctlInt(unix.KEYCTL_UNLINK, e.id, k.keyringID, 0, 0)
	}
	return nil
}

func (k *KeyringCache) IsAvailable() bool {
	// Try to get the session keyring ID; if this fails, the keyring isn't usable.
	_, err := unix.KeyctlGetKeyringID(k.keyringID, false)
	return err == nil
}
