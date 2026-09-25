package browser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext/k6test"
)

func TestInitScriptSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "null",
			input: `null`,
			want:  "",
		},
		{
			name:  "undefined",
			input: `undefined`,
			want:  "",
		},
		{
			name:  "string",
			input: `"Math.random = () => 0"`,
			want:  "Math.random = () => 0",
		},
		{
			name:  "content_object",
			input: `({content: "Math.random = () => 0"})`,
			want:  "Math.random = () => 0",
		},
		{
			name:  "arrow_function",
			input: `() => { Math.random = () => 0 }`,
			want:  "(() => { Math.random = () => 0 })();",
		},
		{
			name:  "function_expression",
			input: `(function() { Math.random = function() { return 0 } })`,
			want:  "(function() { Math.random = function() { return 0 } })();",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			rt := vu.Runtime()
			v, err := rt.RunString(tt.input)
			require.NoError(t, err)

			assert.Equal(t, tt.want, initScriptSource(rt, v))
		})
	}
}
