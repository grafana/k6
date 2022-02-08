package common

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorFromDOMError(t *testing.T) {
	for _, tc := range []struct {
		in       string
		sentinel bool // if it returns the same error value
		want     error
	}{
		{in: "error:timeout", want: ErrTimedOut, sentinel: true},
		{in: "error:notconnected", want: errors.New("element is not attached to the DOM")},
		{in: "error:expectednode:anything", want: errors.New("expected node but got anything")},
		{in: "nonexistent error", want: errors.New("nonexistent error")},
	} {
		got := errorFromDOMError(tc.in)
		if tc.sentinel && !errors.Is(got, tc.want) {
			assert.Failf(t, "not sentinel", "error value of %q should be sentinel", tc.in)
		} else {
			require.Error(t, got)
			assert.EqualError(t, tc.want, got.Error())
		}
	}
}
