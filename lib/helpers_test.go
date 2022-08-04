package lib

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStrictJSONUnmarshal(t *testing.T) {
	t.Parallel()
	type someElement struct {
		Data  int               `json:"data"`
		Props map[string]string `json:"props"`
	}

	testCases := []struct {
		data           string
		expectedError  bool
		destination    interface{}
		expectedResult interface{}
	}{
		{``, true, &someElement{}, nil},
		{`123`, true, &someElement{}, nil},
		{`"blah"`, true, &someElement{}, nil},
		{`null`, false, &someElement{}, &someElement{}},
		{
			`{"data": 123, "props": {"test": "mest"}}`, false, &someElement{},
			&someElement{123, map[string]string{"test": "mest"}},
		},
		{`{"data": 123, "props": {"test": "mest"}}asdg`, true, &someElement{}, nil},
	}
	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("TestCase#%d", i), func(t *testing.T) {
			t.Parallel()
			err := StrictJSONUnmarshal([]byte(tc.data), &tc.destination)
			if tc.expectedError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult, tc.destination)
		})
	}
}

// TODO: test EventStream very thoroughly
