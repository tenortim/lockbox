//go:build linux

package cache

import (
	"sort"
	"testing"
)

func TestKeyringStoreRetrieve(t *testing.T) {
	c := NewKeyringCache()
	if !c.IsAvailable() {
		t.Skip("kernel keyring not available")
	}

	// Clean up any leftover keys.
	c.Clear()
	defer c.Clear()

	if err := c.Store("test_secret", "my-value-123"); err != nil {
		t.Fatalf("Store: %v", err)
	}

	val, err := c.Retrieve("test_secret")
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if val != "my-value-123" {
		t.Errorf("expected 'my-value-123', got %q", val)
	}
}

func TestKeyringRetrieveNotFound(t *testing.T) {
	c := NewKeyringCache()
	if !c.IsAvailable() {
		t.Skip("kernel keyring not available")
	}

	_, err := c.Retrieve("nonexistent_secret_xyz")
	if err == nil {
		t.Error("expected error for nonexistent key")
	}
}

func TestKeyringList(t *testing.T) {
	c := NewKeyringCache()
	if !c.IsAvailable() {
		t.Skip("kernel keyring not available")
	}

	c.Clear()
	defer c.Clear()

	c.Store("alpha", "v1")
	c.Store("beta", "v2")
	c.Store("__env__alpha", "ENV_ALPHA")
	c.Store("__expires__alpha", "2025-12-31T00:00:00Z")

	names, err := c.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	sort.Strings(names)

	// List should NOT include any __ prefixed metadata keys.
	expected := []string{"alpha", "beta"}
	if len(names) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, names)
	}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("index %d: expected %q, got %q", i, expected[i], name)
		}
	}
}

func TestKeyringClear(t *testing.T) {
	c := NewKeyringCache()
	if !c.IsAvailable() {
		t.Skip("kernel keyring not available")
	}

	c.Clear()
	defer c.Clear()

	c.Store("cleartest", "val")
	c.Store("__env__cleartest", "ENV_VAR")

	if err := c.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	names, err := c.List()
	if err != nil {
		t.Fatalf("List after Clear: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("expected 0 keys after clear, got %d: %v", len(names), names)
	}

	// Metadata keys should also be gone.
	_, err = c.Retrieve("__env__cleartest")
	if err == nil {
		t.Error("expected metadata key to be cleared")
	}
}

func TestKeyringOverwrite(t *testing.T) {
	c := NewKeyringCache()
	if !c.IsAvailable() {
		t.Skip("kernel keyring not available")
	}

	c.Clear()
	defer c.Clear()

	c.Store("overwrite_test", "original")
	c.Store("overwrite_test", "updated")

	val, err := c.Retrieve("overwrite_test")
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if val != "updated" {
		t.Errorf("expected 'updated', got %q", val)
	}
}

func TestKeyringIsAvailable(t *testing.T) {
	c := NewKeyringCache()
	// We can't guarantee this will pass in all environments, but on a normal
	// Linux system it should.
	if !c.IsAvailable() {
		t.Skip("kernel keyring not available (expected in some CI environments)")
	}
}
