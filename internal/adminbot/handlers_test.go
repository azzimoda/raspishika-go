package adminbot

import (
	"testing"
	"time"
)

func Test_parseDuration(t *testing.T) {
	tests := []struct {
		name string // description of this test case

		str    string
		want   time.Duration
		wantOk bool
	}{
		{"hour default", "42", time.Hour * 42, true},
		{"hour explicit", "1h", time.Hour, true},
		{"hour explicit spaced", "1 h", time.Hour, true},
		{"day", "1d", time.Hour * 24, true},
		{"week", "1w", time.Hour * 24 * 7, true},
		{"month", "1m", time.Hour * 24 * 30, true},
		{"year", "1y", time.Hour * 24 * 365, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotOk := parseDuration(tt.str)
			if tt.wantOk && gotOk {
				if got != tt.want {
					t.Errorf("parseDuration() = %v, want %v", got, tt.want)
				}
			} else if !tt.wantOk && gotOk {
				t.Errorf("parseDuration() unexpectedly succeeded")
			}
		})
	}
}
