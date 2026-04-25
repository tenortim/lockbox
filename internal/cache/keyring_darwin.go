//go:build darwin

package cache

/*
#cgo LDFLAGS: -framework Security -framework CoreFoundation
#include <Security/Security.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdlib.h>
#include <string.h>

static const char *keychainService = "lockbox";

static inline CFStringRef cfStr(const char *s) {
	return CFStringCreateWithCString(kCFAllocatorDefault, s, kCFStringEncodingUTF8);
}

// storeItem adds or updates a generic password in the default keychain.
static OSStatus storeItem(const char *account, const uint8_t *data, int dataLen) {
	CFStringRef svc  = cfStr(keychainService);
	CFStringRef acct = cfStr(account);
	CFDataRef   blob = CFDataCreate(kCFAllocatorDefault, data, (CFIndex)dataLen);

	const void *qKeys[] = { kSecClass, kSecAttrService, kSecAttrAccount };
	const void *qVals[] = { kSecClassGenericPassword, svc, acct };
	CFDictionaryRef query = CFDictionaryCreate(kCFAllocatorDefault,
		qKeys, qVals, 3,
		&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);

	const void *aKeys[] = { kSecValueData };
	const void *aVals[] = { blob };
	CFDictionaryRef attrs = CFDictionaryCreate(kCFAllocatorDefault,
		aKeys, aVals, 1,
		&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);

	OSStatus status = SecItemUpdate(query, attrs);
	if (status == errSecItemNotFound) {
		const void *addKeys[] = { kSecClass, kSecAttrService, kSecAttrAccount, kSecValueData };
		const void *addVals[] = { kSecClassGenericPassword, svc, acct, blob };
		CFDictionaryRef addDict = CFDictionaryCreate(kCFAllocatorDefault,
			addKeys, addVals, 4,
			&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
		status = SecItemAdd(addDict, NULL);
		CFRelease(addDict);
	}

	CFRelease(attrs);
	CFRelease(query);
	CFRelease(blob);
	CFRelease(acct);
	CFRelease(svc);
	return status;
}

// retrieveItem copies the value for account into a malloc'd buffer.
// On success *outData points to a malloc'd, null-terminated buffer and
// *outLen is the data length (not counting the terminator). The caller
// must zero and free *outData. Returns errSecItemNotFound if absent.
static OSStatus retrieveItem(const char *account, uint8_t **outData, int *outLen) {
	CFStringRef svc  = cfStr(keychainService);
	CFStringRef acct = cfStr(account);

	const void *keys[] = {
		kSecClass, kSecAttrService, kSecAttrAccount, kSecReturnData, kSecMatchLimit,
	};
	const void *vals[] = {
		kSecClassGenericPassword, svc, acct, kCFBooleanTrue, kSecMatchLimitOne,
	};
	CFDictionaryRef query = CFDictionaryCreate(kCFAllocatorDefault,
		keys, vals, 5,
		&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);

	CFTypeRef result = NULL;
	OSStatus status = SecItemCopyMatching(query, &result);
	CFRelease(query);
	CFRelease(acct);
	CFRelease(svc);

	if (status != errSecSuccess) {
		return status;
	}

	CFDataRef data = (CFDataRef)result;
	CFIndex   len  = CFDataGetLength(data);
	*outData = (uint8_t *)malloc((size_t)len + 1);
	if (*outData == NULL) {
		CFRelease(data);
		return -1;
	}
	CFDataGetBytes(data, CFRangeMake(0, len), *outData);
	(*outData)[len] = 0;
	CFRelease(data);
	*outLen = (int)len;
	return errSecSuccess;
}

// deleteAllItems removes every lockbox item from the default keychain.
static OSStatus deleteAllItems(void) {
	CFStringRef svc = cfStr(keychainService);

	const void *keys[] = { kSecClass, kSecAttrService };
	const void *vals[] = { kSecClassGenericPassword, svc };
	CFDictionaryRef query = CFDictionaryCreate(kCFAllocatorDefault,
		keys, vals, 2,
		&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);

	OSStatus status = SecItemDelete(query);
	CFRelease(query);
	CFRelease(svc);
	if (status == errSecItemNotFound) {
		return errSecSuccess;
	}
	return status;
}

// listItems returns all account names under the lockbox service as a
// malloc'd array of malloc'd C strings. *count is set to the array length.
// On errSecItemNotFound, *outNames is NULL and *count is 0 (not an error).
// The caller must free each outNames[i] and then outNames itself.
static OSStatus listItems(char ***outNames, int *count) {
	CFStringRef svc = cfStr(keychainService);

	const void *keys[] = {
		kSecClass, kSecAttrService, kSecReturnAttributes, kSecMatchLimit,
	};
	const void *vals[] = {
		kSecClassGenericPassword, svc, kCFBooleanTrue, kSecMatchLimitAll,
	};
	CFDictionaryRef query = CFDictionaryCreate(kCFAllocatorDefault,
		keys, vals, 4,
		&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	CFRelease(svc);

	CFTypeRef result = NULL;
	OSStatus status = SecItemCopyMatching(query, &result);
	CFRelease(query);

	if (status == errSecItemNotFound) {
		*outNames = NULL;
		*count    = 0;
		return errSecSuccess;
	}
	if (status != errSecSuccess) {
		return status;
	}

	CFArrayRef arr = (CFArrayRef)result;
	CFIndex n = CFArrayGetCount(arr);

	char **names = (char **)malloc(sizeof(char *) * (size_t)n);
	if (names == NULL) {
		CFRelease(arr);
		return -1;
	}

	for (CFIndex i = 0; i < n; i++) {
		CFDictionaryRef dict = (CFDictionaryRef)CFArrayGetValueAtIndex(arr, i);
		CFStringRef acct = (CFStringRef)CFDictionaryGetValue(dict, kSecAttrAccount);
		CFIndex maxLen = CFStringGetMaximumSizeForEncoding(
			CFStringGetLength(acct), kCFStringEncodingUTF8) + 1;
		names[i] = (char *)malloc((size_t)maxLen);
		if (names[i] != NULL) {
			CFStringGetCString(acct, names[i], maxLen, kCFStringEncodingUTF8);
		}
	}

	CFRelease(arr);
	*outNames = names;
	*count    = (int)n;
	return errSecSuccess;
}
*/
import "C"
import (
	"fmt"
	"strings"
	"unsafe"
)

const internalPrefix = "__"

// KeychainCache implements SessionCache using the macOS Keychain.
// Items are stored as generic passwords under the "lockbox" service;
// the service name provides namespace isolation so no key prefix is needed.
// Both NewKeyringCache and NewUserKeyringCache return the same backend —
// macOS has no equivalent of Linux's session-scoped keyring; items persist
// for the lifetime of the user's login keychain and are automatically
// encrypted by the Keychain under the user's credentials.
type KeychainCache struct{}

func NewKeyringCache() *KeychainCache     { return &KeychainCache{} }
func NewUserKeyringCache() *KeychainCache { return &KeychainCache{} }

func (k *KeychainCache) Store(name, value string) error {
	cAcct := C.CString(name)
	defer C.free(unsafe.Pointer(cAcct))

	data := []byte(value)
	var dataPtr *C.uint8_t
	if len(data) > 0 {
		dataPtr = (*C.uint8_t)(&data[0])
	}

	status := C.storeItem(cAcct, dataPtr, C.int(len(data)))
	if status != C.errSecSuccess {
		return fmt.Errorf("keychain store: OSStatus %d", status)
	}
	return nil
}

func (k *KeychainCache) Retrieve(name string) (string, error) {
	cAcct := C.CString(name)
	defer C.free(unsafe.Pointer(cAcct))

	var outData *C.uint8_t
	var outLen C.int

	status := C.retrieveItem(cAcct, &outData, &outLen)
	if status == C.errSecItemNotFound {
		return "", fmt.Errorf("secret '%s' not found in session cache", name)
	}
	if status != C.errSecSuccess {
		return "", fmt.Errorf("keychain retrieve: OSStatus %d", status)
	}
	if outData == nil || outLen == 0 {
		return "", nil
	}

	buf := C.GoBytes(unsafe.Pointer(outData), outLen)
	val := string(buf)
	for i := range buf {
		buf[i] = 0
	}
	C.memset(unsafe.Pointer(outData), 0, C.size_t(outLen))
	C.free(unsafe.Pointer(outData))
	return val, nil
}

func (k *KeychainCache) List() ([]string, error) {
	all, err := k.listRaw()
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

func (k *KeychainCache) listRaw() ([]string, error) {
	var cNames **C.char
	var count C.int

	status := C.listItems(&cNames, &count)
	if status != C.errSecSuccess {
		return nil, fmt.Errorf("keychain list: OSStatus %d", status)
	}
	if count == 0 || cNames == nil {
		return nil, nil
	}
	defer func() {
		slice := unsafe.Slice(cNames, count)
		for _, s := range slice {
			C.free(unsafe.Pointer(s))
		}
		C.free(unsafe.Pointer(cNames))
	}()

	slice := unsafe.Slice(cNames, count)
	result := make([]string, int(count))
	for i, s := range slice {
		result[i] = C.GoString(s)
	}
	return result, nil
}

func (k *KeychainCache) Clear() error {
	status := C.deleteAllItems()
	if status != C.errSecSuccess {
		return fmt.Errorf("keychain clear: OSStatus %d", status)
	}
	return nil
}

// IsAvailable reports whether the Keychain is accessible. It probes with a
// known-absent key: errSecItemNotFound means the Keychain responded normally.
func (k *KeychainCache) IsAvailable() bool {
	cAcct := C.CString("__probe__")
	defer C.free(unsafe.Pointer(cAcct))
	var outData *C.uint8_t
	var outLen C.int
	status := C.retrieveItem(cAcct, &outData, &outLen)
	if outData != nil {
		C.free(unsafe.Pointer(outData))
	}
	return status == C.errSecSuccess || status == C.errSecItemNotFound
}
