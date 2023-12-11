package tests

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/xk6-browser/common"
)

func TestConsoleLogParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		log  string
		want string
	}{
		{
			name: "number", log: "1", want: "1",
		},
		{
			name: "string", log: `"some string"`, want: "some string",
		},
		{
			name: "bool", log: "true", want: "true",
		},
		{
			name: "empty_array", log: "[]", want: "{}", // TODO: Improve this output
		},
		{
			name: "empty_object", log: "{}", want: "{}",
		},
		{
			name: "filled_object", log: `{"foo":{"bar1":"bar2"}}`, want: `{"foo":"Object"}`,
		},
		{
			name: "filled_array", log: `["foo","bar"]`, want: `{"0":"foo","1":"bar"}`,
		},
		{
			name: "filled_array", log: `() => true`, want: `function()`,
		},
		{
			name: "empty", log: "", want: "",
		},
		{
			name: "null", log: "null", want: "null",
		},
		{
			name: "undefined", log: "undefined", want: "undefined",
		},
		{
			name: "bigint", log: `BigInt("2")`, want: "2n",
		},
		{
			name: "unwrapped_bigint", log: "3n", want: "3n",
		},
		{
			name: "float", log: "3.14", want: "3.14",
		},
		{
			name: "scientific_notation", log: "123e-5", want: "0.00123",
		},
		{
			name: "partially_parsed",
			log:  "window",
			want: `{"document":"#document","location":"Location","name":"","self":"Window","window":"Window"}`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())
			p := tb.NewPage(nil)

			done := make(chan bool)

			eventHandler := func(cm *common.ConsoleMessage) {
				defer close(done)
				assert.Equal(t, tt.want, cm.Text)
			}

			// eventHandler will be called from a separate goroutine from within
			// the page's async event loop. This is why we need to wait on done
			// to close.
			err := p.On("console", eventHandler)
			require.NoError(t, err)

			if tt.log == "" {
				p.Evaluate(tb.toGojaValue(`() => console.log("")`))
			} else {
				p.Evaluate(tb.toGojaValue(fmt.Sprintf("() => console.log(%s)", tt.log)))
			}

			var (
				assertTO bool
				testTO   = 2500 * time.Millisecond
			)

			select {
			case <-done:
			case <-time.After(testTO):
				assertTO = true
			}

			assert.False(t, assertTO, "test timed out before event handlers were called")
		})
	}
}
