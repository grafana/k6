package browser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext/k6test"
)

func TestParseMouseClickOptions(t *testing.T) {
	t.Parallel()

	t.Run("null_returns_defaults", func(t *testing.T) {
		t.Parallel()
		vu := k6test.NewVU(t)
		v, err := vu.Runtime().RunString(`null`)
		require.NoError(t, err)

		opts, err := parseMouseClickOptions(vu.Runtime(), v)
		require.NoError(t, err)
		assert.Equal(t, "left", opts.Button)
		assert.Equal(t, int64(1), opts.ClickCount)
		assert.Equal(t, int64(0), opts.Delay)
	})

	t.Run("undefined_returns_defaults", func(t *testing.T) {
		t.Parallel()
		vu := k6test.NewVU(t)
		v, err := vu.Runtime().RunString(`undefined`)
		require.NoError(t, err)

		opts, err := parseMouseClickOptions(vu.Runtime(), v)
		require.NoError(t, err)
		assert.Equal(t, "left", opts.Button)
		assert.Equal(t, int64(1), opts.ClickCount)
		assert.Equal(t, int64(0), opts.Delay)
	})

	t.Run("parses_all_options", func(t *testing.T) {
		t.Parallel()
		vu := k6test.NewVU(t)
		v, err := vu.Runtime().RunString(`({button: "right", clickCount: 3, delay: 100})`)
		require.NoError(t, err)

		opts, err := parseMouseClickOptions(vu.Runtime(), v)
		require.NoError(t, err)
		assert.Equal(t, "right", opts.Button)
		assert.Equal(t, int64(3), opts.ClickCount)
		assert.Equal(t, int64(100), opts.Delay)
	})

	t.Run("parses_partial_options", func(t *testing.T) {
		t.Parallel()
		vu := k6test.NewVU(t)
		v, err := vu.Runtime().RunString(`({button: "middle"})`)
		require.NoError(t, err)

		opts, err := parseMouseClickOptions(vu.Runtime(), v)
		require.NoError(t, err)
		assert.Equal(t, "middle", opts.Button)
		assert.Equal(t, int64(1), opts.ClickCount)
		assert.Equal(t, int64(0), opts.Delay)
	})

	t.Run("invalid_clickCount_type_returns_error", func(t *testing.T) {
		t.Parallel()
		vu := k6test.NewVU(t)
		v, err := vu.Runtime().RunString(`({clickCount: "invalid"})`)
		require.NoError(t, err)

		_, err = parseMouseClickOptions(vu.Runtime(), v)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "clickCount must be a number")
	})

	t.Run("invalid_delay_type_returns_error", func(t *testing.T) {
		t.Parallel()
		vu := k6test.NewVU(t)
		v, err := vu.Runtime().RunString(`({delay: true})`)
		require.NoError(t, err)

		_, err = parseMouseClickOptions(vu.Runtime(), v)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "delay must be a number")
	})
}

func TestParseMouseDblClickOptions(t *testing.T) {
	t.Parallel()

	t.Run("null_returns_defaults", func(t *testing.T) {
		t.Parallel()
		vu := k6test.NewVU(t)
		v, err := vu.Runtime().RunString(`null`)
		require.NoError(t, err)

		opts, err := parseMouseDblClickOptions(vu.Runtime(), v)
		require.NoError(t, err)
		assert.Equal(t, "left", opts.Button)
		assert.Equal(t, int64(0), opts.Delay)
	})

	t.Run("parses_all_options", func(t *testing.T) {
		t.Parallel()
		vu := k6test.NewVU(t)
		v, err := vu.Runtime().RunString(`({button: "right", delay: 50})`)
		require.NoError(t, err)

		opts, err := parseMouseDblClickOptions(vu.Runtime(), v)
		require.NoError(t, err)
		assert.Equal(t, "right", opts.Button)
		assert.Equal(t, int64(50), opts.Delay)
	})

	t.Run("invalid_delay_type_returns_error", func(t *testing.T) {
		t.Parallel()
		vu := k6test.NewVU(t)
		v, err := vu.Runtime().RunString(`({delay: "slow"})`)
		require.NoError(t, err)

		_, err = parseMouseDblClickOptions(vu.Runtime(), v)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "delay must be a number")
	})
}

func TestParseMouseDownUpOptions(t *testing.T) {
	t.Parallel()

	t.Run("null_returns_defaults", func(t *testing.T) {
		t.Parallel()
		vu := k6test.NewVU(t)
		v, err := vu.Runtime().RunString(`null`)
		require.NoError(t, err)

		opts, err := parseMouseDownUpOptions(vu.Runtime(), v)
		require.NoError(t, err)
		assert.Equal(t, "left", opts.Button)
		assert.Equal(t, int64(1), opts.ClickCount)
	})

	t.Run("parses_all_options", func(t *testing.T) {
		t.Parallel()
		vu := k6test.NewVU(t)
		v, err := vu.Runtime().RunString(`({button: "middle", clickCount: 2})`)
		require.NoError(t, err)

		opts, err := parseMouseDownUpOptions(vu.Runtime(), v)
		require.NoError(t, err)
		assert.Equal(t, "middle", opts.Button)
		assert.Equal(t, int64(2), opts.ClickCount)
	})

	t.Run("invalid_clickCount_type_returns_error", func(t *testing.T) {
		t.Parallel()
		vu := k6test.NewVU(t)
		v, err := vu.Runtime().RunString(`({clickCount: []})`)
		require.NoError(t, err)

		_, err = parseMouseDownUpOptions(vu.Runtime(), v)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "clickCount must be a number")
	})
}

func TestParseMouseMoveOptions(t *testing.T) {
	t.Parallel()

	t.Run("null_returns_defaults", func(t *testing.T) {
		t.Parallel()
		vu := k6test.NewVU(t)
		v, err := vu.Runtime().RunString(`null`)
		require.NoError(t, err)

		opts, err := parseMouseMoveOptions(vu.Runtime(), v)
		require.NoError(t, err)
		assert.Equal(t, int64(1), opts.Steps)
	})

	t.Run("parses_steps_option", func(t *testing.T) {
		t.Parallel()
		vu := k6test.NewVU(t)
		v, err := vu.Runtime().RunString(`({steps: 10})`)
		require.NoError(t, err)

		opts, err := parseMouseMoveOptions(vu.Runtime(), v)
		require.NoError(t, err)
		assert.Equal(t, int64(10), opts.Steps)
	})

	t.Run("invalid_steps_type_returns_error", func(t *testing.T) {
		t.Parallel()
		vu := k6test.NewVU(t)
		v, err := vu.Runtime().RunString(`({steps: {}})`)
		require.NoError(t, err)

		_, err = parseMouseMoveOptions(vu.Runtime(), v)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "steps must be a number")
	})
}
