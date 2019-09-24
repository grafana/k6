package lib

import (
	"strings"
	"testing"
	"time"
)

func TestTimeoutError(t *testing.T) {
	tests := []struct {
		stage, expectedStrContain string
		d                         time.Duration
	}{
		{"setup", "1 seconds", time.Second},
		{"teardown", "2 seconds", time.Second * 2},
		{"", "0 seconds", time.Duration(0)},
	}

	for _, tc := range tests {
		te := NewTimeoutError(tc.stage, tc.d)
		if !strings.Contains(te.String(), tc.expectedStrContain) {
			t.Errorf("Expected error contains %s, but got: %s", tc.expectedStrContain, te.String())
		}
	}
}

func TestTimeoutErrorHint(t *testing.T) {
	tests := []struct {
		stage string
		empty bool
	}{
		{"setup", false},
		{"teardown", false},
		{"not handle", true},
	}

	for _, tc := range tests {
		te := NewTimeoutError(tc.stage, time.Second)
		if tc.empty && te.Hint() != "" {
			t.Errorf("Expected empty hint, got: %s", te.Hint())
		}
		if !tc.empty && te.Hint() == "" {
			t.Errorf("Expected non-empty hint, got empty")
		}
	}
}
