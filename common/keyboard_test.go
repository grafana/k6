package common

import (
	"testing"

	"github.com/grafana/xk6-browser/k6ext/k6test"

	"github.com/stretchr/testify/assert"
)

func TestSplit(t *testing.T) {
	type args struct {
		keys string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "empty slice on empty string",
			args: args{
				keys: "",
			},
			want: []string{""},
		},
		{
			name: "empty slice on string without separator",
			args: args{
				keys: "HelloWorld!",
			},
			want: []string{"HelloWorld!"},
		},
		{
			name: "string split with separator",
			args: args{
				keys: "Hello+World+!",
			},
			want: []string{"Hello", "World", "!"},
		},
		{
			name: "do not split on single +",
			args: args{
				keys: "+",
			},
			want: []string{"+"},
		},
		{
			name: "split ++ to + and ''",
			args: args{
				keys: "++",
			},
			want: []string{"+", ""},
		},
		{
			name: "split +++ to + and +",
			args: args{
				keys: "+++",
			},
			want: []string{"+", "+"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := split(tt.args.keys)

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestKeyboardPress(t *testing.T) {
	t.Run("panics when '' empty key passed in", func(t *testing.T) {
		vu := k6test.NewVU(t)
		k := NewKeyboard(vu.Context(), nil)
		assert.Panics(t, func() { k.Press("", nil) })
	})
}
