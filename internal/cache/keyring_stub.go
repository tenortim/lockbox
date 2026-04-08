//go:build !linux

package cache

import "errors"

var errUnsupported = errors.New("session cache not supported on this platform (Linux kernel keyring required)")

// StubCache is a no-op implementation for non-Linux platforms.
type StubCache struct{}

func NewKeyringCache() *StubCache     { return &StubCache{} }
func NewUserKeyringCache() *StubCache { return &StubCache{} }

func (s *StubCache) Store(name, value string) error     { return errUnsupported }
func (s *StubCache) Retrieve(name string) (string, error) { return "", errUnsupported }
func (s *StubCache) List() ([]string, error)             { return nil, errUnsupported }
func (s *StubCache) Clear() error                        { return errUnsupported }
func (s *StubCache) IsAvailable() bool                   { return false }
