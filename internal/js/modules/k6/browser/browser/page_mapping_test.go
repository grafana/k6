package browser

import (
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"
)

func TestParseStringOrRegex(t *testing.T) {
	t.Parallel()

	rt := sobek.New()
	mk := func(code string) sobek.Value {
		v, err := rt.RunString(code)
		require.NoError(t, err)
		return v
	}

	tests := []struct {
		name        string
		input       sobek.Value
		doubleQuote bool
		want        string
	}{
		{name: "string_single_quote", input: mk(`'abc'`), doubleQuote: false, want: `'abc'`},
		{name: "string_single_quote", input: mk(`'abc'`), doubleQuote: true, want: `"abc"`},
		{name: "string_double_quote", input: mk(`"abc"`), doubleQuote: true, want: `"abc"`},
		{name: "string_double_quote", input: mk(`"abc"`), doubleQuote: false, want: `'abc'`},
		{name: "regex_literal", input: mk(`/ab+c/i`), doubleQuote: false, want: `/ab+c/i`},
		{name: "number", input: mk(`123`), doubleQuote: true, want: `123`},
		{name: "boolean", input: mk(`true`), doubleQuote: false, want: `true`},
		{name: "object", input: mk(`({a:1})`), doubleQuote: false, want: `[object Object]`},
		{name: "null", input: mk(`null`), doubleQuote: false, want: `null`},
		{name: "undefined", input: mk(`undefined`), doubleQuote: false, want: `undefined`},
		{name: "undefined", input: mk(``), doubleQuote: false, want: `undefined`},
		{name: "string_with_single_quote", input: mk(`'abc\''`), doubleQuote: false, want: `'abc\''`},
		{name: "string_with_single_quote", input: mk(`'abc\''`), doubleQuote: true, want: `"abc'"`},
		{name: "string_with_double_quote", input: mk(`'abc"'`), doubleQuote: false, want: `'abc"'`},
		{name: "string_with_double_quote", input: mk(`'abc"'`), doubleQuote: true, want: `"abc\""`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := parseStringOrRegex(tc.input, tc.doubleQuote)
			require.Equal(t, tc.want, got)
		})
	}
}
