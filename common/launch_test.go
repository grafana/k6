package common

import (
	"context"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLaunchOptionsParse(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		opts   map[string]interface{}
		assert func(*testing.T, *LaunchOptions)
	}{
		// TODO: Check other options.
		{
			name: "args",
			opts: map[string]interface{}{
				"args": []interface{}{"browser-arg1='value1", "browser-arg2=value2", "browser-flag"},
			},
			assert: func(t *testing.T, lopts *LaunchOptions) {
				require.Len(t, lopts.Args, 3)
				assert.Equal(t, "browser-arg1='value1", lopts.Args[0])
				assert.Equal(t, "browser-arg2=value2", lopts.Args[1])
				assert.Equal(t, "browser-flag", lopts.Args[2])
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rt := goja.New()
			opts := rt.ToValue(tc.opts)
			lopts := NewLaunchOptions()
			err := lopts.Parse(context.Background(), opts)
			require.NoError(t, err)
			tc.assert(t, lopts)
		})
	}
}
