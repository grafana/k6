package faker

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_register(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		register()
	})
}
