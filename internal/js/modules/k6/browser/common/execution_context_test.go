package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsUnserializableRemoteObjectError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		msg  string
		want bool
	}{
		{
			name: "reference chain too long",
			msg:  "Object reference chain is too long",
			want: true,
		},
		{
			name: "could not return by value",
			msg:  "Object couldn't be returned by value",
			want: true,
		},
		{
			name: "inspected target closed",
			msg:  "Inspected target navigated or closed",
			want: false,
		},
		{
			name: "empty",
			msg:  "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, isUnserializableRemoteObjectError(tt.msg))
		})
	}
}
