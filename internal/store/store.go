package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"filippo.io/age"
	"filippo.io/age/armor"
)

const (
	storeDirPerms  = 0700
	storeFilePerms = 0600
)

var (
	ErrStoreExists      = errors.New("store already exists")
	ErrStoreNotFound    = errors.New("store not found")
	ErrWrongPassword    = errors.New("wrong master password")
	ErrDuplicateName    = errors.New("secret with that name already exists")
	ErrDuplicateEnvVar  = errors.New("another secret already maps to that environment variable")
	ErrSecretNotFound   = errors.New("secret not found")
)

func DefaultStorePath() string {
	if dir := os.Getenv("LOCKBOX_STORE"); dir != "" {
		return dir
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(configDir, "lockbox", "store.age")
}

func Create(path, passphrase string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, storeDirPerms); err != nil {
		return fmt.Errorf("creating store directory: %w", err)
	}
	if _, err := os.Stat(path); err == nil {
		return ErrStoreExists
	}
	return Save(path, passphrase, NewStoreData())
}

func Open(path, passphrase string) (*StoreData, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrStoreNotFound
		}
		return nil, fmt.Errorf("opening store: %w", err)
	}
	defer f.Close()

	identity, err := age.NewScryptIdentity(passphrase)
	if err != nil {
		return nil, fmt.Errorf("creating identity: %w", err)
	}

	armorReader := armor.NewReader(f)
	reader, err := age.Decrypt(armorReader, identity)
	if err != nil {
		return nil, ErrWrongPassword
	}

	var data StoreData
	if err := json.NewDecoder(reader).Decode(&data); err != nil {
		return nil, fmt.Errorf("decoding store: %w", err)
	}
	if data.Secrets == nil {
		data.Secrets = make(map[string]*Secret)
	}
	return &data, nil
}

func Save(path, passphrase string, data *StoreData) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, storeDirPerms); err != nil {
		return fmt.Errorf("creating store directory: %w", err)
	}

	var buf bytes.Buffer
	armorWriter := armor.NewWriter(&buf)

	recipient, err := age.NewScryptRecipient(passphrase)
	if err != nil {
		return fmt.Errorf("creating recipient: %w", err)
	}

	writer, err := age.Encrypt(armorWriter, recipient)
	if err != nil {
		return fmt.Errorf("encrypting: %w", err)
	}

	if err := json.NewEncoder(writer).Encode(data); err != nil {
		return fmt.Errorf("encoding store: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("closing age writer: %w", err)
	}
	if err := armorWriter.Close(); err != nil {
		return fmt.Errorf("closing armor writer: %w", err)
	}

	// Atomic write: write to temp file, then rename.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), storeFilePerms); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}

// Lock acquires an exclusive flock on the store file's parent directory.
// Returns an unlock function that must be called when done.
func Lock(path string) (unlock func(), err error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, storeDirPerms); err != nil {
		return nil, fmt.Errorf("creating store directory: %w", err)
	}
	lockPath := filepath.Join(dir, ".lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("opening lock file: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("acquiring lock: %w", err)
	}
	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, nil
}

// AddSecret adds a secret to the store data, validating for duplicates.
func (d *StoreData) AddSecret(name string, secret *Secret) error {
	if _, exists := d.Secrets[name]; exists {
		return ErrDuplicateName
	}
	for n, s := range d.Secrets {
		if s.EnvVar == secret.EnvVar && n != name {
			return fmt.Errorf("%w: '%s' already uses %s", ErrDuplicateEnvVar, n, secret.EnvVar)
		}
	}
	d.Secrets[name] = secret
	return nil
}

// RemoveSecret removes a secret from the store data.
func (d *StoreData) RemoveSecret(name string) error {
	if _, exists := d.Secrets[name]; !exists {
		return ErrSecretNotFound
	}
	delete(d.Secrets, name)
	return nil
}

// ZeroBytes overwrites a byte slice with zeros.
func ZeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// ReadAll reads all content from a reader. The caller is responsible for
// zeroing the returned slice when done (use ZeroBytes).
func ReadAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}
