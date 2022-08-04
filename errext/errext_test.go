package errext

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/errext/exitcodes"
)

func assertHasHint(t *testing.T, err error, hint string) {
	var typederr HasHint
	require.ErrorAs(t, err, &typederr)
	assert.Equal(t, typederr.Hint(), hint)
	assert.Contains(t, err.Error(), typederr.Error())
}

func assertHasExitCode(t *testing.T, err error, exitcode exitcodes.ExitCode) {
	var typederr HasExitCode
	require.ErrorAs(t, err, &typederr)
	assert.Equal(t, typederr.ExitCode(), exitcode)
	assert.Contains(t, err.Error(), typederr.Error())
}

func TestErrextHelpers(t *testing.T) {
	t.Parallel()

	const testExitCode exitcodes.ExitCode = 13
	assert.Nil(t, WithHint(nil, "test hint"))
	assert.Nil(t, WithExitCodeIfNone(nil, testExitCode))

	errBase := errors.New("base error")
	errBaseWithHint := WithHint(errBase, "test hint")
	assertHasHint(t, errBaseWithHint, "test hint")
	errBaseWithTwoHints := WithHint(errBaseWithHint, "better hint")
	assertHasHint(t, errBaseWithTwoHints, "better hint (test hint)")

	errWrapperWithHints := fmt.Errorf("wrapper error: %w", errBaseWithTwoHints)
	assertHasHint(t, errWrapperWithHints, "better hint (test hint)")

	errWithExitCode := WithExitCodeIfNone(errWrapperWithHints, testExitCode)
	assertHasHint(t, errWithExitCode, "better hint (test hint)")
	assertHasExitCode(t, errWithExitCode, testExitCode)

	errWithExitCodeAgain := WithExitCodeIfNone(errWithExitCode, exitcodes.ExitCode(27))
	assertHasHint(t, errWithExitCodeAgain, "better hint (test hint)")
	assertHasExitCode(t, errWithExitCodeAgain, testExitCode)

	errBaseWithThreeHints := WithHint(errWithExitCodeAgain, "best hint")
	assertHasHint(t, errBaseWithThreeHints, "best hint (better hint (test hint))")

	finalErrorMess := fmt.Errorf("woot: %w", errBaseWithThreeHints)
	assert.Equal(t, finalErrorMess.Error(), "woot: wrapper error: base error")
	assertHasHint(t, finalErrorMess, "best hint (better hint (test hint))")
	assertHasExitCode(t, finalErrorMess, testExitCode)
}
