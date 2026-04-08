package store

import (
	"fmt"
	"time"
)

const ExpiryWarningDays = 14

type Secret struct {
	Value       string     `json:"value"`
	EnvVar      string     `json:"env_var"`
	Description string     `json:"description,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

// ExpiryStatus returns a human-readable expiry status for the secret.
// Returns "" if no expiry is set.
func (s *Secret) ExpiryStatus() string {
	if s.ExpiresAt == nil {
		return ""
	}
	remaining := time.Until(*s.ExpiresAt)
	days := int(remaining.Hours() / 24)
	switch {
	case remaining <= 0:
		return "EXPIRED"
	case days == 0:
		return "expires today"
	case days == 1:
		return "expires tomorrow"
	case days <= ExpiryWarningDays:
		return fmt.Sprintf("expires in %d days", days)
	default:
		return s.ExpiresAt.Format("2006-01-02")
	}
}

// IsExpired returns true if the secret has an expiry date in the past.
func (s *Secret) IsExpired() bool {
	return s.ExpiresAt != nil && time.Now().After(*s.ExpiresAt)
}

// IsExpiringSoon returns true if the secret expires within ExpiryWarningDays.
func (s *Secret) IsExpiringSoon() bool {
	if s.ExpiresAt == nil {
		return false
	}
	return time.Until(*s.ExpiresAt) <= time.Duration(ExpiryWarningDays)*24*time.Hour
}

type StoreData struct {
	Secrets map[string]*Secret `json:"secrets"`
}

func NewStoreData() *StoreData {
	return &StoreData{Secrets: make(map[string]*Secret)}
}
