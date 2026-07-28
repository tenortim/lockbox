package cmd

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tenortim/lockbox/internal/store"
)

type fakeSessionCache struct {
	available bool
	entries   map[string]string
	listErr   error
	clearErr  error
	storeErr  map[string]error
}

func (f *fakeSessionCache) Store(name, value string) error {
	if err := f.storeErr[name]; err != nil {
		return err
	}
	f.entries[name] = value
	return nil
}

func (f *fakeSessionCache) Retrieve(name string) (string, error) {
	value, ok := f.entries[name]
	if !ok {
		return "", fmt.Errorf("not found")
	}
	return value, nil
}

func (f *fakeSessionCache) List() ([]string, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var names []string
	for name := range f.entries {
		if !strings.HasPrefix(name, "__") {
			names = append(names, name)
		}
	}
	return names, nil
}

func (f *fakeSessionCache) Clear() error {
	if f.clearErr != nil {
		return f.clearErr
	}
	clear(f.entries)
	return nil
}

func (f *fakeSessionCache) IsAvailable() bool {
	return f.available
}

func TestSessionCacheUnlocked(t *testing.T) {
	tests := []struct {
		name      string
		cache     *fakeSessionCache
		want      bool
		wantError bool
	}{
		{
			name:  "unavailable",
			cache: &fakeSessionCache{},
		},
		{
			name: "empty",
			cache: &fakeSessionCache{
				available: true,
				entries:   map[string]string{},
			},
		},
		{
			name: "unlocked",
			cache: &fakeSessionCache{
				available: true,
				entries:   map[string]string{"token": "value"},
			},
			want: true,
		},
		{
			name: "list error",
			cache: &fakeSessionCache{
				available: true,
				entries:   map[string]string{},
				listErr:   errors.New("list failed"),
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sessionCacheUnlocked(tt.cache)
			if (err != nil) != tt.wantError {
				t.Fatalf("sessionCacheUnlocked() error = %v, wantError %v", err, tt.wantError)
			}
			if got != tt.want {
				t.Errorf("sessionCacheUnlocked() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRefreshSessionCacheReplacesContents(t *testing.T) {
	expiresAt := time.Date(2027, 3, 25, 0, 0, 0, 0, time.UTC)
	c := &fakeSessionCache{
		available: true,
		entries: map[string]string{
			"old_token":             "old-value",
			"__env__old_token":      "OLD_TOKEN",
			"kept_token":            "stale-value",
			"__expires__kept_token": "2026-01-01T00:00:00Z",
		},
	}
	data := &store.StoreData{
		Secrets: map[string]*store.Secret{
			"kept_token": {
				Value:       "new-value",
				EnvVar:      "KEPT_TOKEN",
				Description: "updated description",
			},
			"new_token": {
				Value:       "added-value",
				EnvVar:      "NEW_TOKEN",
				Description: "new description",
				ExpiresAt:   &expiresAt,
			},
		},
	}

	if err := refreshSessionCache(c, data); err != nil {
		t.Fatalf("refreshSessionCache(): %v", err)
	}

	want := map[string]string{
		"kept_token":           "new-value",
		"__env__kept_token":    "KEPT_TOKEN",
		"__desc__kept_token":   "updated description",
		"new_token":            "added-value",
		"__env__new_token":     "NEW_TOKEN",
		"__desc__new_token":    "new description",
		"__expires__new_token": expiresAt.Format(time.RFC3339),
	}
	if !reflect.DeepEqual(c.entries, want) {
		t.Errorf("cache entries = %#v, want %#v", c.entries, want)
	}
}

func TestRefreshSessionCacheClearsPartialCacheOnFailure(t *testing.T) {
	c := &fakeSessionCache{
		available: true,
		entries:   map[string]string{"stale": "value"},
		storeErr:  map[string]error{"__env__token": errors.New("store failed")},
	}
	data := &store.StoreData{
		Secrets: map[string]*store.Secret{
			"token": {
				Value:  "value",
				EnvVar: "TOKEN",
			},
		},
	}

	err := refreshSessionCache(c, data)
	if err == nil {
		t.Fatal("refreshSessionCache() returned nil error")
	}
	if len(c.entries) != 0 {
		t.Errorf("cache entries = %#v, want empty cache", c.entries)
	}
}
