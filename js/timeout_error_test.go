package js

import (
	"strings"
	"testing"
	"time"

	"go.k6.io/k6/lib/consts"
)

func TestTimeoutError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		stage, expectedStrContain string
		d                         time.Duration
	}{
		{consts.SetupFn, "1 seconds", time.Second},
		{consts.TeardownFn, "2 seconds", time.Second * 2},
		{"", "0 seconds", time.Duration(0)},
	}

	for _, tc := range tests {
		te := newTimeoutError(tc.stage, tc.d)
		if !strings.Contains(te.Error(), tc.expectedStrContain) {
			t.Errorf("Expected error contains %s, but got: %s", tc.expectedStrContain, te.Error())
		}
	}
}

func TestTimeoutErrorHint(t *testing.T) {
	t.Parallel()
	tests := []struct {
		stage string
		empty bool
	}{
		{consts.SetupFn, false},
		{consts.TeardownFn, false},
		{"not handle", true},
	}

	for _, tc := range tests {
		te := newTimeoutError(tc.stage, time.Second)
		if tc.empty && te.Hint() != "" {
			t.Errorf("Expected empty hint, got: %s", te.Hint())
		}
		if !tc.empty && te.Hint() == "" {
			t.Errorf("Expected non-empty hint, got empty")
		}
	}
}
