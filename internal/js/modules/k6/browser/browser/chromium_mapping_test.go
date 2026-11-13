package browser

import (
	"testing"
	"time"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseConnectOverCDPOptions checks that the optional trailing options
// argument of chromium.connectOverCDP is parsed correctly.
func TestParseConnectOverCDPOptions(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		o, err := parseConnectOverCDPOptions(sobek.New(), nil)
		require.NoError(t, err)
		assert.Zero(t, o.Timeout)
	})

	t.Run("undefined", func(t *testing.T) {
		t.Parallel()
		o, err := parseConnectOverCDPOptions(sobek.New(), sobek.Undefined())
		require.NoError(t, err)
		assert.Zero(t, o.Timeout)
	})

	t.Run("timeout in ms", func(t *testing.T) {
		t.Parallel()
		rt := sobek.New()
		obj := rt.NewObject()
		require.NoError(t, obj.Set("timeout", 5000))
		o, err := parseConnectOverCDPOptions(rt, obj)
		require.NoError(t, err)
		assert.Equal(t, 5*time.Second, o.Timeout)
	})
}
