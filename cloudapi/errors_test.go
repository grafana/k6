package cloudapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContains(t *testing.T) {
	t.Parallel()

	s := []string{"a", "b", "c"}

	assert.False(t, contains(s, "e"))
	assert.True(t, contains(s, "b"))
}

func TestErrorResponse_Error(t *testing.T) {
	t.Parallel()

	msg1 := "some message"
	msg2 := "some other message"

	errResp := ResponseError{
		Message: msg1,
		Errors:  []string{msg2},
		FieldErrors: map[string][]string{
			"field1": {"error1", "error2"},
		},
		Code: 123,
	}

	expected := "(E123) " + msg1 + "\n " + msg2 + "\n field1: error1, error2"
	assert.Equal(t, expected, errResp.Error())
}
