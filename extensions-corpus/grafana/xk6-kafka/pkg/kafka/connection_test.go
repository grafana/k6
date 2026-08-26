package kafka

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/js/modulestest"
)

// TestConnectionExposesClose verifies the Go Close method is reachable from JS
// as `close()` via k6's field-name mapper (the same mapper the real runtime
// uses), matching the index.d.ts contract.
func TestConnectionExposesClose(t *testing.T) {
	t.Parallel()

	rt := modulestest.NewRuntime(t)
	require.NoError(t, rt.VU.Runtime().Set("conn", &Connection{}))

	v, err := rt.VU.Runtime().RunString(`typeof conn.close`)
	require.NoError(t, err)
	require.Equal(t, "function", v.String())
}
