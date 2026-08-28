package kafka

import (
	"testing"

	"github.com/stretchr/testify/require"
	extensionapitest "go.k6.io/k6-extension-api/test"
)

// TestConnectionExposesClose verifies the Go Close method is reachable from JS
// as `close()` via k6's field-name mapper (the same mapper the real runtime
// uses), matching the index.d.ts contract.
func TestConnectionExposesClose(t *testing.T) {
	t.Parallel()

	vu := extensionapitest.NewVU()
	require.NoError(t, vu.Runtime().Set("conn", &Connection{}))

	v, err := vu.Runtime().RunString(`typeof conn.close`)
	require.NoError(t, err)
	require.Equal(t, "function", v.String())
}
