package lib

import (
	"testing"
	"time"
)

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
