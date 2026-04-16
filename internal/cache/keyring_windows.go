//go:build windows

package cache

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// keyPrefix and internalPrefix mirror the Linux implementation's conventions
// so that metadata keys (__env__*, __expires__*) are stored and filtered
// identically across platforms.
const (
	keyPrefix      = "lockbox:"
	internalPrefix = "__"
)

// Windows Credential Manager constants.
const (
	credTypeGeneric         = 1 // CRED_TYPE_GENERIC
	credPersistLocalMachine = 2 // CRED_PERSIST_LOCAL_MACHINE
	//
	// CRED_PERSIST_LOCAL_MACHINE is chosen to match KEY_SPEC_USER_KEYRING on
	// Linux: credentials survive process exits and SSH/RDP reconnects for the
	// same user account and are only removed by an explicit "lockbox lock" or
	// by a password change. Unlike the Linux user keyring they are NOT
	// automatically wiped when the last user session ends, so "lockbox lock"
	// is more important on Windows. All data is DPAPI-encrypted under the
	// current user's master key, so other Windows accounts cannot read it.
)

var (
	advapi32           = windows.NewLazySystemDLL("advapi32.dll")
	procCredWriteW     = advapi32.NewProc("CredWriteW")
	procCredReadW      = advapi32.NewProc("CredReadW")
	procCredDeleteW    = advapi32.NewProc("CredDeleteW")
	procCredEnumerateW = advapi32.NewProc("CredEnumerateW")
	procCredFree       = advapi32.NewProc("CredFree")
)

// winCREDENTIAL mirrors the Win32 CREDENTIALW structure. Field layout must
// match the C struct exactly; unsafe.Pointer arithmetic depends on it.
type winCREDENTIAL struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	_                  [4]byte // padding: LPBYTE must be pointer-aligned on 64-bit
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

// CredManagerCache implements SessionCache using the Windows Credential Manager.
// Credentials are stored as CRED_TYPE_GENERIC entries with the "lockbox:" prefix
// and encrypted at rest by DPAPI scoped to the current user account.
type CredManagerCache struct{}

func NewKeyringCache() *CredManagerCache     { return &CredManagerCache{} }
func NewUserKeyringCache() *CredManagerCache { return &CredManagerCache{} }

// Store writes name→value into the Credential Manager, overwriting any
// existing entry with the same name.
func (c *CredManagerCache) Store(name, value string) error {
	targetName, err := windows.UTF16PtrFromString(keyPrefix + name)
	if err != nil {
		return fmt.Errorf("encoding credential name: %w", err)
	}

	blob := []byte(value)
	cred := winCREDENTIAL{
		Type:               credTypeGeneric,
		TargetName:         targetName,
		CredentialBlobSize: uint32(len(blob)),
		Persist:            credPersistLocalMachine,
	}
	if len(blob) > 0 {
		cred.CredentialBlob = &blob[0]
	}

	r, _, e := procCredWriteW.Call(uintptr(unsafe.Pointer(&cred)), 0)
	if r == 0 {
		return fmt.Errorf("CredWriteW: %w", e)
	}
	return nil
}

// Retrieve reads the value stored under name from the Credential Manager.
func (c *CredManagerCache) Retrieve(name string) (string, error) {
	targetName, err := windows.UTF16PtrFromString(keyPrefix + name)
	if err != nil {
		return "", fmt.Errorf("encoding credential name: %w", err)
	}

	var pcred *winCREDENTIAL
	r, _, e := procCredReadW.Call(
		uintptr(unsafe.Pointer(targetName)),
		uintptr(credTypeGeneric),
		0,
		uintptr(unsafe.Pointer(&pcred)),
	)
	if r == 0 {
		if isNotFound(e) {
			return "", fmt.Errorf("secret '%s' not found in session cache", name)
		}
		return "", fmt.Errorf("CredReadW: %w", e)
	}
	defer procCredFree.Call(uintptr(unsafe.Pointer(pcred)))

	if pcred.CredentialBlobSize == 0 {
		return "", nil
	}

	// Copy out of Credential Manager memory before freeing.
	blob := make([]byte, pcred.CredentialBlobSize)
	src := unsafe.Slice(pcred.CredentialBlob, pcred.CredentialBlobSize)
	copy(blob, src)
	val := string(blob)
	for i := range blob {
		blob[i] = 0
	}
	return val, nil
}

// List returns the names of user-visible secrets (excludes internal __ keys).
func (c *CredManagerCache) List() ([]string, error) {
	all, err := c.listRaw()
	if err != nil {
		return nil, err
	}
	var names []string
	for _, name := range all {
		if !strings.HasPrefix(name, internalPrefix) {
			names = append(names, name)
		}
	}
	return names, nil
}

// listRaw returns all lockbox entry names including internal metadata keys.
func (c *CredManagerCache) listRaw() ([]string, error) {
	filterPtr, err := windows.UTF16PtrFromString(keyPrefix + "*")
	if err != nil {
		return nil, fmt.Errorf("encoding filter: %w", err)
	}

	var count uint32
	// pcreds receives a PCREDENTIAL* (pointer to an array of CREDENTIAL pointers)
	// written by the Windows API. Keeping it as unsafe.Pointer rather than uintptr
	// avoids the uintptr→unsafe.Pointer conversion that go vet flags as unsafe.
	var pcreds unsafe.Pointer
	r, _, e := procCredEnumerateW.Call(
		uintptr(unsafe.Pointer(filterPtr)),
		0,
		uintptr(unsafe.Pointer(&count)),
		uintptr(unsafe.Pointer(&pcreds)),
	)
	if r == 0 {
		if isNotFound(e) {
			return nil, nil // no lockbox credentials exist yet
		}
		return nil, fmt.Errorf("CredEnumerateW: %w", e)
	}
	defer procCredFree.Call(uintptr(pcreds))

	// Interpret pcreds as a pointer to an array of count unsafe.Pointer values,
	// each of which points to a CREDENTIAL structure. Using unsafe.Pointer
	// elements avoids a uintptr→unsafe.Pointer round-trip.
	credPtrs := unsafe.Slice((*unsafe.Pointer)(pcreds), count)
	names := make([]string, 0, count)
	for _, p := range credPtrs {
		if p == nil {
			continue
		}
		cred := (*winCREDENTIAL)(p)
		if cred.TargetName == nil {
			continue
		}
		target := windows.UTF16PtrToString(cred.TargetName)
		names = append(names, strings.TrimPrefix(target, keyPrefix))
	}
	return names, nil
}

// Clear removes all lockbox credentials from the Credential Manager.
func (c *CredManagerCache) Clear() error {
	names, err := c.listRaw()
	if err != nil {
		return err
	}
	for _, name := range names {
		targetName, err := windows.UTF16PtrFromString(keyPrefix + name)
		if err != nil {
			continue
		}
		procCredDeleteW.Call(
			uintptr(unsafe.Pointer(targetName)),
			uintptr(credTypeGeneric),
			0,
		)
	}
	return nil
}

// IsAvailable reports whether the Credential Manager API is reachable.
// On any supported Windows version this is always true; the check guards
// against hypothetical stripped/sandboxed environments.
func (c *CredManagerCache) IsAvailable() bool {
	return procCredReadW.Find() == nil
}

// isNotFound returns true when the Windows error indicates the credential
// does not exist (ERROR_NOT_FOUND = 1168 / 0x490).
func isNotFound(err error) bool {
	var errno syscall.Errno
	return errors.As(err, &errno) && errno == 0x490
}
