package store

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestCreateAndOpen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.age")
	passphrase := "test-master-password"

	if err := Create(path, passphrase); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// File should exist with restricted permissions (Unix only; Windows does not
	// enforce octal permission bits).
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0600 {
			t.Errorf("expected perms 0600, got %04o", perm)
		}
	}

	// Open with correct password.
	data, err := Open(path, passphrase)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(data.Secrets) != 0 {
		t.Errorf("expected empty secrets, got %d", len(data.Secrets))
	}

	// Open with wrong password should fail.
	_, err = Open(path, "wrong-password")
	if err != ErrWrongPassword {
		t.Errorf("expected ErrWrongPassword, got %v", err)
	}
}

func TestCreateAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.age")

	if err := Create(path, "pw"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := Create(path, "pw"); err != ErrStoreExists {
		t.Errorf("expected ErrStoreExists, got %v", err)
	}
}

func TestOpenNotFound(t *testing.T) {
	_, err := Open("/nonexistent/path/store.age", "pw")
	if err != ErrStoreNotFound {
		t.Errorf("expected ErrStoreNotFound, got %v", err)
	}
}

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.age")
	pw := "test-pw"

	if err := Create(path, pw); err != nil {
		t.Fatalf("Create: %v", err)
	}

	data, err := Open(path, pw)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	secret := &Secret{
		Value:       "ghp_test1234567890",
		EnvVar:      "GITHUB_TOKEN",
		Description: "Test token",
		CreatedAt:   now,
	}
	if err := data.AddSecret("github_pat", secret); err != nil {
		t.Fatalf("AddSecret: %v", err)
	}

	if err := Save(path, pw, data); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Re-open and verify.
	data2, err := Open(path, pw)
	if err != nil {
		t.Fatalf("Open after save: %v", err)
	}
	s, ok := data2.Secrets["github_pat"]
	if !ok {
		t.Fatal("secret 'github_pat' not found after round-trip")
	}
	if s.Value != "ghp_test1234567890" {
		t.Errorf("value mismatch: got %q", s.Value)
	}
	if s.EnvVar != "GITHUB_TOKEN" {
		t.Errorf("env_var mismatch: got %q", s.EnvVar)
	}
	if s.Description != "Test token" {
		t.Errorf("description mismatch: got %q", s.Description)
	}
}

func TestAddDuplicateName(t *testing.T) {
	data := NewStoreData()
	s := &Secret{Value: "v", EnvVar: "ENV1", CreatedAt: time.Now()}
	if err := data.AddSecret("a", s); err != nil {
		t.Fatal(err)
	}
	s2 := &Secret{Value: "v2", EnvVar: "ENV2", CreatedAt: time.Now()}
	if err := data.AddSecret("a", s2); err != ErrDuplicateName {
		t.Errorf("expected ErrDuplicateName, got %v", err)
	}
}

func TestAddDuplicateEnvVar(t *testing.T) {
	data := NewStoreData()
	s := &Secret{Value: "v", EnvVar: "SAME_VAR", CreatedAt: time.Now()}
	if err := data.AddSecret("a", s); err != nil {
		t.Fatal(err)
	}
	s2 := &Secret{Value: "v2", EnvVar: "SAME_VAR", CreatedAt: time.Now()}
	err := data.AddSecret("b", s2)
	if err == nil {
		t.Fatal("expected duplicate env var error")
	}
}

func TestRemoveSecret(t *testing.T) {
	data := NewStoreData()
	s := &Secret{Value: "v", EnvVar: "ENV1", CreatedAt: time.Now()}
	if err := data.AddSecret("a", s); err != nil {
		t.Fatal(err)
	}
	if err := data.RemoveSecret("a"); err != nil {
		t.Fatal(err)
	}
	if len(data.Secrets) != 0 {
		t.Errorf("expected 0 secrets, got %d", len(data.Secrets))
	}
}

func TestRemoveSecretNotFound(t *testing.T) {
	data := NewStoreData()
	if err := data.RemoveSecret("nope"); err != ErrSecretNotFound {
		t.Errorf("expected ErrSecretNotFound, got %v", err)
	}
}

func TestMultipleSecrets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.age")
	pw := "pw"

	if err := Create(path, pw); err != nil {
		t.Fatal(err)
	}

	data, _ := Open(path, pw)
	for i, name := range []string{"github", "jira", "confluence"} {
		s := &Secret{
			Value:     "secret-" + name,
			EnvVar:    "TOKEN_" + name,
			CreatedAt: time.Now().Add(time.Duration(i) * time.Second),
		}
		if err := data.AddSecret(name, s); err != nil {
			t.Fatal(err)
		}
	}
	if err := Save(path, pw, data); err != nil {
		t.Fatal(err)
	}

	data2, err := Open(path, pw)
	if err != nil {
		t.Fatal(err)
	}
	if len(data2.Secrets) != 3 {
		t.Errorf("expected 3 secrets, got %d", len(data2.Secrets))
	}
}

func TestLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.age")

	// Ensure the directory exists for the lock file.
	unlock, err := Lock(path)
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}
	unlock()
}

func TestDefaultStorePath(t *testing.T) {
	// With LOCKBOX_STORE set.
	t.Setenv("LOCKBOX_STORE", "/custom/path/store.age")
	if got := DefaultStorePath(); got != "/custom/path/store.age" {
		t.Errorf("expected /custom/path/store.age, got %s", got)
	}
}

func TestDirectoryPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not enforce Unix octal directory permission bits")
	}
	dir := t.TempDir()
	storeDir := filepath.Join(dir, "lockbox")
	path := filepath.Join(storeDir, "store.age")

	if err := Create(path, "pw"); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0700 {
		t.Errorf("expected dir perms 0700, got %04o", perm)
	}
}

func TestExpiryStatus(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt *time.Time
		wantEmpty bool
		want      string
	}{
		{"nil expiry", nil, true, ""},
		{"expired", timePtr(time.Now().Add(-24 * time.Hour)), false, "EXPIRED"},
		{"today", timePtr(time.Now().Add(6 * time.Hour)), false, "expires today"},
		{"tomorrow", timePtr(time.Now().Add(30 * time.Hour)), false, "expires tomorrow"},
		{"soon", timePtr(time.Now().Add(7 * 24 * time.Hour)), false, ""},      // contains "expires in"
		{"far out", timePtr(time.Now().Add(90 * 24 * time.Hour)), false, ""},  // date format
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Secret{ExpiresAt: tt.expiresAt}
			got := s.ExpiryStatus()
			if tt.wantEmpty && got != "" {
				t.Errorf("expected empty, got %q", got)
			}
			if !tt.wantEmpty && got == "" {
				t.Error("expected non-empty expiry status")
			}
			if tt.want != "" && got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestIsExpired(t *testing.T) {
	s := &Secret{ExpiresAt: timePtr(time.Now().Add(-1 * time.Hour))}
	if !s.IsExpired() {
		t.Error("expected IsExpired=true for past date")
	}

	s2 := &Secret{ExpiresAt: timePtr(time.Now().Add(24 * time.Hour))}
	if s2.IsExpired() {
		t.Error("expected IsExpired=false for future date")
	}

	s3 := &Secret{}
	if s3.IsExpired() {
		t.Error("expected IsExpired=false for nil expiry")
	}
}

func TestIsExpiringSoon(t *testing.T) {
	s := &Secret{ExpiresAt: timePtr(time.Now().Add(3 * 24 * time.Hour))}
	if !s.IsExpiringSoon() {
		t.Error("expected IsExpiringSoon=true for 3 days out")
	}

	s2 := &Secret{ExpiresAt: timePtr(time.Now().Add(30 * 24 * time.Hour))}
	if s2.IsExpiringSoon() {
		t.Error("expected IsExpiringSoon=false for 30 days out")
	}

	s3 := &Secret{}
	if s3.IsExpiringSoon() {
		t.Error("expected IsExpiringSoon=false for nil expiry")
	}
}

func TestExpiryRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.age")
	pw := "pw"

	if err := Create(path, pw); err != nil {
		t.Fatal(err)
	}

	data, _ := Open(path, pw)
	expires := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	s := &Secret{
		Value:     "v",
		EnvVar:    "E",
		CreatedAt: time.Now().UTC(),
		ExpiresAt: &expires,
	}
	data.AddSecret("test", s)
	Save(path, pw, data)

	data2, _ := Open(path, pw)
	got := data2.Secrets["test"]
	if got.ExpiresAt == nil {
		t.Fatal("expected ExpiresAt to be set after round-trip")
	}
	if !got.ExpiresAt.Equal(expires) {
		t.Errorf("expected %v, got %v", expires, *got.ExpiresAt)
	}
}

func timePtr(t time.Time) *time.Time { return &t }
