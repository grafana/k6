package js

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The file is mostly around test that have different behaviour in ESM and commonJS

func TestReturnInCommonJSModule(t *testing.T) {
	t.Parallel()
	script := `
	if (false) {
		return // this works as this gets wrapped in a function
	}
	exports.default = () => {}
	`
	_, err := getSimpleRunner(t, "/script.js", script)
	require.NoError(t, err)
}

func TestReturnInESMModule(t *testing.T) {
	t.Parallel()
	script := `
	if (false) {
		return // this is syntax error
	}
	export default = () => {}
	`
	_, err := getSimpleRunner(t, "/script.js", script)
	require.ErrorContains(t, err, "Illegal return statement")
}
