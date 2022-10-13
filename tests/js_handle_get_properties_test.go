package tests

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJSHandleGetProperties(t *testing.T) {
	tb := newTestBrowser(t)
	p := tb.NewPage(nil)

	handle := p.EvaluateHandle(tb.toGojaValue(`
	() => {
		return {
			prop1: "one",
			prop2: "two",
			prop3: "three"
		};
	}
	`))

	props := handle.GetProperties()
	value := props["prop1"].JSONValue().String()
	assert.Equal(t, value, "one", `expected property value of "one", got %q`, value)
}
