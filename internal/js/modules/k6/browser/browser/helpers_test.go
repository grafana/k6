package browser

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext/k6test"
	"go.k6.io/k6/v2/js/modulestest"
)

func TestPromiseRejectsWithJavaScriptError(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		err     error
		message string
	}{
		{"plain", errors.New("navigation failed"), "navigation failed"},
		{"wrapped", fmt.Errorf("navigating: %w", context.Canceled), "navigating: context canceled"},
		{"timeout", &k6ext.UserFriendlyError{Err: context.DeadlineExceeded, Timeout: time.Second}, "timed out after 1s"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			runtime := modulestest.NewRuntime(t)
			vu := moduleVU{VU: runtime.VU}
			require.NoError(t, vu.Runtime().Set("operation", func() *sobek.Promise {
				return promise(vu, func() (any, error) { return nil, tc.err })
			}))

			value, err := runtime.RunOnEventLoop(`(async () => {
				try {
					await operation();
				} catch (error) {
					return [error.name, error.message, error instanceof Error];
				}
			})()`)
			require.NoError(t, err)
			result := k6test.ToPromise(t, value)
			require.Equal(t, sobek.PromiseStateFulfilled, result.State())
			assert.Equal(t, []any{"GoError", tc.message, true}, result.Result().Export())
		})
	}
}

func TestSobekEmptyString(t *testing.T) {
	t.Parallel()
	// SobekEmpty string should return true if the argument
	// is an empty string or not defined in the Sobek runtime.
	rt := sobek.New()
	require.NoError(t, rt.Set("sobekEmptyString", sobekEmptyString))
	for _, s := range []string{"() => true", "'() => false'"} { // not empty
		v, err := rt.RunString(`sobekEmptyString(` + s + `)`)
		require.NoError(t, err)
		require.Falsef(t, v.ToBoolean(), "got: true, want: false for %q", s)
	}
	for _, s := range []string{"", "  ", "null", "undefined"} { // empty
		v, err := rt.RunString(`sobekEmptyString(` + s + `)`)
		require.NoError(t, err)
		require.Truef(t, v.ToBoolean(), "got: false, want: true for %q", s)
	}
}
