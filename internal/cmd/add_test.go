package cmd

import (
	"testing"
	"time"
)

func TestParseExpiry_Duration(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		input    string
		minDelta time.Duration
		maxDelta time.Duration
	}{
		{"30d", 29 * 24 * time.Hour, 31 * 24 * time.Hour},
		{"30D", 29 * 24 * time.Hour, 31 * 24 * time.Hour},
		{"4w", 27 * 24 * time.Hour, 29 * 24 * time.Hour},
		{"4W", 27 * 24 * time.Hour, 29 * 24 * time.Hour},
		{"1y", 364 * 24 * time.Hour, 367 * 24 * time.Hour},
		{"1Y", 364 * 24 * time.Hour, 367 * 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseExpiry(tt.input)
			if err != nil {
				t.Fatalf("parseExpiry(%q): %v", tt.input, err)
			}
			delta := got.Sub(now)
			if delta < tt.minDelta || delta > tt.maxDelta {
				t.Errorf("parseExpiry(%q): delta %v not in [%v, %v]", tt.input, delta, tt.minDelta, tt.maxDelta)
			}
		})
	}
}

func TestParseExpiry_Date(t *testing.T) {
	tests := []struct {
		input string
		want  time.Time
	}{
		{"2025-06-15", time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)},
		{"2025-12-31T23:59:59Z", time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseExpiry(tt.input)
			if err != nil {
				t.Fatalf("parseExpiry(%q): %v", tt.input, err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestParseExpiry_Invalid(t *testing.T) {
	invalids := []string{"", "abc", "30", "30x", "not-a-date", "2025/06/15"}
	for _, s := range invalids {
		t.Run(s, func(t *testing.T) {
			_, err := parseExpiry(s)
			if err == nil {
				t.Errorf("expected error for %q", s)
			}
		})
	}
}
