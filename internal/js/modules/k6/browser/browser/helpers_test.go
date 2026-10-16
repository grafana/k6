package browser

import (
	"sync/atomic"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext/k6test"
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

// TestPromiseInInitContext ensures that promise() rejects, instead of running
// fn, when called in the init context. fn is where the browser APIs dereference
// VU.State(), and it runs in a plain goroutine, so a nil State there is an
// unrecovered panic that takes the whole k6 process down. See #6178.
func TestPromiseInInitContext(t *testing.T) {
	t.Parallel()

	// A VU that has not been activated has a nil State: the init context.
	vu := k6test.NewVU(t)

	// fn deliberately doesn't touch State, so a missing guard shows up as
	// "fn ran" instead of as a crashed test binary.
	var ran atomic.Bool
	require.NoError(t, vu.Runtime().Set("initContextPromise", func() *sobek.Promise {
		return promise(moduleVU{VU: vu}, func() (any, error) {
			ran.Store(true)
			return nil, nil
		})
	}))

	p := vu.RunPromise(t, `
		try {
			await initContextPromise();
			return "no error";
		} catch (e) {
			return String(e);
		}
	`)
	require.Equal(t, sobek.PromiseStateFulfilled, p.State())
	require.Contains(t, p.Result().String(), errInitContext.Error())
	require.False(t, ran.Load(), "fn must not run in the init context")
}
