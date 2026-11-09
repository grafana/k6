package provisioning

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/errext"
)

// abortErr constructs an error with an attached AbortReason,
// matching the pattern used elsewhere in k6's test suite.
func abortErr(reason errext.AbortReason, msg string) error {
	return errext.WithAbortReasonIfNone(errors.New(msg), reason)
}

func TestMapTestErrorToNotifyCode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name               string
		err                error
		wantNil            bool
		wantCode           int32
		wantReasonContains string
	}{
		{"nil returns nil", nil, true, 0, ""},
		{"AbortedByUser → 8036", abortErr(errext.AbortedByUser, "user pressed Ctrl+C"), false, 8036, "user"},
		{"AbortedByScriptAbort → 8036", abortErr(errext.AbortedByScriptAbort, "abort()"), false, 8036, "abort"},
		{"AbortedByScriptError → 8035", abortErr(errext.AbortedByScriptError, "TypeError"), false, 8035, "TypeError"},
		{"AbortedByOutput → 8034", abortErr(errext.AbortedByOutput, "cloud push failed"), false, 8034, "cloud"},
		{"AbortedByTimeout → 8034", abortErr(errext.AbortedByTimeout, "timeout"), false, 8034, "timeout"},
		{"AbortedByThreshold → 8036", abortErr(errext.AbortedByThreshold, "threshold crossed"), false, 8036, "threshold"},
		{"AbortedByThresholdsAfterTestEnd → nil (test completed)", abortErr(errext.AbortedByThresholdsAfterTestEnd, "post-end"), true, 0, ""},
		{"generic error → 0 (Unknown)", errors.New("something else"), false, 0, "something else"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := mapTestErrorToNotifyCode(tc.err)

			if tc.wantNil {
				assert.Nil(t, got)
				return
			}

			require.NotNil(t, got, "expected non-nil notifyError")
			assert.Equal(t, tc.wantCode, got.Code)
			assert.Contains(t, got.Reason, tc.wantReasonContains)
		})
	}
}
