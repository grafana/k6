package browser

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext"
	"go.k6.io/k6/v2/js/modulestest"
)

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

func TestJSErrorProperties(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		err         error
		wantName    string
		wantMessage string
	}{
		{
			name:        "generic",
			err:         errors.New("boom"),
			wantName:    "Error",
			wantMessage: "boom",
		},
		{
			name: "timeout",
			err: fmt.Errorf("waiting for navigation: %w", &k6ext.UserFriendlyError{
				Err:     context.DeadlineExceeded,
				Timeout: 500 * time.Millisecond,
			}),
			wantName:    "TimeoutError",
			wantMessage: "waiting for navigation: timed out after 500ms",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rt := sobek.New()
			obj := jsError(rt, tc.err)
			require.Equal(t, tc.wantName, obj.Get("name").String())
			require.Equal(t, tc.wantMessage, obj.Get("message").String())
		})
	}
}

func TestPromiseRejectsJSErrorProperties(t *testing.T) {
	t.Parallel()

	runtime := modulestest.NewRuntime(t)
	vu := moduleVU{VU: runtime.VU}

	err := runtime.EventLoop.Start(func() error {
		if err := runtime.VU.Runtime().Set("pTimeout", promise(vu, func() (any, error) {
			return nil, fmt.Errorf("waiting for navigation: %w", &k6ext.UserFriendlyError{
				Err:     context.DeadlineExceeded,
				Timeout: time.Second,
			})
		})); err != nil {
			return err
		}
		if err := runtime.VU.Runtime().Set("pGeneric", promise(vu, func() (any, error) {
			return nil, errors.New("click failed")
		})); err != nil {
			return err
		}

		_, err := runtime.VU.Runtime().RunString(`
			pTimeout.then(
				() => { throw new Error("expected timeout rejection"); },
				(e) => {
					if (e.name !== "TimeoutError") {
						throw new Error("timeout name=" + e.name);
					}
					if (!String(e.message).includes("timed out after 1s")) {
						throw new Error("timeout message=" + e.message);
					}
				}
			).then(() => pGeneric).then(
				() => { throw new Error("expected generic rejection"); },
				(e) => {
					if (e.name !== "Error") {
						throw new Error("generic name=" + e.name);
					}
					if (e.message !== "click failed") {
						throw new Error("generic message=" + e.message);
					}
				}
			)
		`)
		return err
	})
	runtime.EventLoop.WaitOnRegistered()
	require.NoError(t, err)
}
