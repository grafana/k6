/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2021 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

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
