package common

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestColorSchemeJSON(t *testing.T) {
	t.Parallel()

	testOptionEnumJSON(t, map[string]ColorScheme{
		"light":         ColorSchemeLight,
		"dark":          ColorSchemeDark,
		"no-preference": ColorSchemeNoPreference,
	}, ColorSchemeDark, ColorScheme("invalid"), "to ColorScheme")
}

func TestReducedMotionJSON(t *testing.T) {
	t.Parallel()

	testOptionEnumJSON(t, map[string]ReducedMotion{
		"reduce":        ReducedMotionReduce,
		"no-preference": ReducedMotionNoPreference,
	}, ReducedMotionReduce, ReducedMotion("invalid"), "to ReducedMotion")
}

func TestDOMElementStateJSON(t *testing.T) {
	t.Parallel()

	testOptionEnumJSON(t, map[string]DOMElementState{
		"attached": DOMElementStateAttached,
		"detached": DOMElementStateDetached,
		"visible":  DOMElementStateVisible,
		"hidden":   DOMElementStateHidden,
	}, DOMElementStateHidden, DOMElementState(-1), "unmarshaling DOM element state")
}

func TestImageFormatJSON(t *testing.T) {
	t.Parallel()

	testOptionEnumJSON(t, map[string]ImageFormat{
		"jpeg": ImageFormatJPEG,
		"png":  ImageFormatPNG,
	}, ImageFormatPNG, ImageFormat("invalid"), "unmarshalling image format")
}

func TestPollingTypeJSON(t *testing.T) {
	t.Parallel()

	testOptionEnumJSON(t, map[string]PollingType{
		"raf":      PollingRaf,
		"mutation": PollingMutation,
		"interval": PollingInterval,
	}, PollingMutation, PollingType(-1), "unmarshaling polling type")
}

func testOptionEnumJSON[T interface {
	comparable
	fmt.Stringer
}](t *testing.T, values map[string]T, initial, invalid T, errorContext string) {
	t.Helper()

	for text, value := range values {
		t.Run(text, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, text, value.String())

			encoded, err := json.Marshal(value)
			require.NoError(t, err)
			assert.Equal(t, `"`+text+`"`, string(encoded))

			decoded := initial
			require.NoError(t, json.Unmarshal(encoded, &decoded))
			assert.Equal(t, value, decoded)
		})
	}

	t.Run("unknown_value", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, invalid.String())
		encoded, err := json.Marshal(invalid)
		require.NoError(t, err)
		assert.Equal(t, `""`, string(encoded))
	})

	// Unknown strings and null reset these enums to their zero value.
	for _, input := range []string{`"unknown"`, `""`, `null`} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			decoded := initial
			require.NoError(t, json.Unmarshal([]byte(input), &decoded))
			var zero T
			assert.Equal(t, zero, decoded)
		})
	}

	for _, input := range []string{`1`, `true`, `[]`, `{}`} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			decoded := initial
			err := json.Unmarshal([]byte(input), &decoded)
			require.ErrorContains(t, err, errorContext)
			var typeErr *json.UnmarshalTypeError
			require.ErrorAs(t, err, &typeErr)
			assert.Equal(t, initial, decoded)
		})
	}
}
