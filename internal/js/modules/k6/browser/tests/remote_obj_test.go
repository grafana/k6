package tests

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
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
			name: "empty_array", log: "[]", want: "[]",
		},
		{
			name: "empty_object", log: "{}", want: "{}",
		},
		{
			name: "filled_object", log: `{"foo":{"bar1":"bar2"}}`, want: `{"foo":"Object"}`,
		},
		{
			name: "filled_array", log: `["foo","bar"]`, want: `["foo","bar"]`,
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

			eventHandler := func(event common.PageOnEvent) error {
				defer close(done)
				assert.Equal(t, tt.want, event.ConsoleMessage.Text)
				return nil
			}

			// eventHandler will be called from a separate goroutine from within
			// the page's async event loop. This is why we need to wait on done
			// to close.
			err := p.On("console", eventHandler)
			require.NoError(t, err)

			if tt.log == "" {
				_, err = p.Evaluate(`() => console.log("")`)
			} else {
				_, err = p.Evaluate(fmt.Sprintf("() => console.log(%s)", tt.log))
			}
			require.NoError(t, err)

			select {
			case <-done:
			case <-time.After(2500 * time.Millisecond):
				assert.Fail(t, "test timed out before event handler was called")
			}
		})
	}
}

func TestEvalRemoteObjectParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		eval string
		want any
	}{
		{
			name: "number", eval: "1", want: float64(1),
		},
		{
			name: "string", eval: `"some string"`, want: "some string",
		},
		{
			name: "bool", eval: "true", want: true,
		},
		{
			name: "empty_array", eval: "[]", want: []any{},
		},
		{
			name: "empty_object", eval: "{}", want: nil,
		},
		{
			name: "filled_object", eval: `{return {foo:"bar"};}`, want: map[string]any{"foo": "bar"},
		},
		{
			name: "filled_array", eval: `{return ["foo","bar"];}`, want: []any{0: "foo", 1: "bar"},
		},
		{
			name: "filled_array", eval: `() => true`, want: `function()`,
		},
		{
			name: "empty", eval: "", want: "",
		},
		{
			name: "null", eval: "null", want: nil,
		},
		{
			name: "undefined", eval: "undefined", want: nil,
		},
		{
			name: "bigint", eval: `BigInt("2")`, want: int64(2),
		},
		{
			name: "unwrapped_bigint", eval: "3n", want: int64(3),
		},
		{
			name: "float", eval: "3.14", want: 3.14,
		},
		{
			name: "scientific_notation", eval: "123e-5", want: 0.00123,
		},
		// TODO:
		// {
		// 	// This test is ignored until https://go.k6.io/k6/js/modules/k6/browser/issues/1132
		// 	// has been resolved.
		// 	name: "partially_parsed",
		// 	eval:  "window",
		// 	want: `{"document":"#document","location":"Location","name":"","self":"Window","window":"Window"}`,
		// },
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())
			p := tb.NewPage(nil)

			var (
				got any
				err error
			)
			if tt.eval == "" {
				got, err = p.Evaluate(`() => ""`)
			} else {
				got, err = p.Evaluate(fmt.Sprintf("() => %s", tt.eval))
			}
			require.NoError(t, err)
			assert.EqualValues(t, tt.want, got)
		})
	}
}
