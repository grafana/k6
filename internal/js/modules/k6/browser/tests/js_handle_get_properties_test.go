package tests

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSHandleGetProperties(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)
	p := tb.NewPage(nil)

	handle, err := p.EvaluateHandle(`
	() => {
		return {
			prop1: "one",
			prop2: "two",
			prop3: "three",
			get accessor() { return "from-getter"; }
		};
	}
	`)
	require.NoError(t, err, "expected no error when evaluating handle")

	props, err := handle.GetProperties()
	require.NoError(t, err, "expected no error when getting properties")

	value, err := props["prop1"].JSONValue()
	assert.NoError(t, err, "expected no error when getting JSONValue")
	assert.Equal(t, value, "one", `expected property value of "one", got %q`, value)

	_, hasAccessor := props["accessor"]
	assert.False(t, hasAccessor, "enumerable getters have no CDP value and must be skipped, not panic")
}
