package js

import (
	"fmt"
	goruntime "runtime"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"
)

// TestWeakMapDeleteReuseKeepsLiveEntries guards against sobek WeakMap ABA:
// delete(key) must Stop the key's cleanup, otherwise a later GC of that key can
// wipe a live entry whose Object was allocated at the same address.
//
// Regression for the sobek WeakMap rewrite landed in #6218.
func TestWeakMapDeleteReuseKeepsLiveEntries(t *testing.T) {
	t.Parallel()

	rt := sobek.New()
	_, err := rt.RunString(`globalThis.__wm = new WeakMap();`)
	require.NoError(t, err)

	const rounds = 20000
	for round := range rounds {
		_, err := rt.RunString(`
(() => {
  let a = {};
  __wm.set(a, "OLD");
  __wm.delete(a);
  a = null;
})()`)
		require.NoError(t, err)

		goruntime.GC()

		_, err = rt.RunString(`
(() => {
  const b = {};
  __wm.set(b, "NEW");
  globalThis.__last = b;
})()`)
		require.NoError(t, err)

		goruntime.GC()
		time.Sleep(200 * time.Microsecond)

		val, err := rt.RunString(`__wm.get(__last)`)
		require.NoError(t, err)
		require.Equal(t, "NEW", val.String(),
			"live WeakMap entry lost after delete+GC address reuse at round %d (got %v)", round, val)

		_, err = rt.RunString(`__wm.delete(__last); __last = null;`)
		require.NoError(t, err)
	}
}

// TestWeakSetDeleteReuseKeepsLiveEntries is the WeakSet counterpart of the
// WeakMap ABA regression above (WeakSet shares the same weakMap storage).
func TestWeakSetDeleteReuseKeepsLiveEntries(t *testing.T) {
	t.Parallel()

	rt := sobek.New()
	_, err := rt.RunString(`globalThis.__ws = new WeakSet();`)
	require.NoError(t, err)

	const rounds = 20000
	for round := range rounds {
		_, err := rt.RunString(`
(() => {
  let a = {};
  __ws.add(a);
  __ws.delete(a);
  a = null;
})()`)
		require.NoError(t, err)

		goruntime.GC()

		_, err = rt.RunString(`
(() => {
  const b = {};
  __ws.add(b);
  globalThis.__last = b;
})()`)
		require.NoError(t, err)

		goruntime.GC()
		time.Sleep(200 * time.Microsecond)

		val, err := rt.RunString(`__ws.has(__last)`)
		require.NoError(t, err)
		require.True(t, val.ToBoolean(),
			fmt.Sprintf("live WeakSet membership lost after delete+GC address reuse at round %d", round))

		_, err = rt.RunString(`__ws.delete(__last); __last = null;`)
		require.NoError(t, err)
	}
}
