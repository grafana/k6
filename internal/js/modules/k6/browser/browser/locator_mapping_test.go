package browser

import (
	"fmt"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"
)

func TestParseLocatorOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "string", text: `'hello'`, want: `"hello"i`},
		{name: "string_with_separator", text: `'a >> b'`, want: `"a >> b"i`},
		{name: "string_with_slashes", text: `'/api/users'`, want: `"/api/users"i`},
		{name: "string_with_double_quotes", text: `'"Done"'`, want: `"\"Done\""i`},
		{name: "string_with_single_quote", text: `"it's"`, want: `"it's"i`},
		{name: "string_with_backslash", text: `'C:\\temp'`, want: `"C:\\temp"i`},
		{name: "string_with_html", text: `'a & <b>'`, want: `"a & <b>"i`},
		{name: "empty_string", text: `''`, want: ``},
		{name: "number", text: `42`, want: `"42"i`},
		{name: "regex", text: `/^Done$/`, want: `/^Done$/`},
		{name: "regex_with_flags", text: `/done/i`, want: `/done/i`},
		{name: "regex_with_double_quotes", text: `/"Done"/`, want: `/\"Done\"/`},
		{name: "regex_with_single_quote", text: `/it's/`, want: `/it\'s/`},
		{name: "regex_with_backtick", text: "/`a`/", want: "/\\`a\\`/"},
		{name: "regex_with_adjacent_quotes", text: `/["']/`, want: `/[\"\']/`},
		{name: "regex_with_escaped_quote", text: `/\"a/`, want: `/\"a/`},
		{name: "regex_with_escaped_backslash", text: `/\\"/`, want: `/\\\"/`},
		{name: "regex_with_separator", text: `/a >> b/`, want: `/a \>\> b/`},
		{name: "unicode_regex", text: `/"a" >> b/u`, want: `/"a" >> b/u`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rt := sobek.New()
			opts, err := rt.RunString(fmt.Sprintf(`({hasText: %[1]s, hasNotText: %[1]s})`, tt.text))
			require.NoError(t, err)

			got := parseLocatorOptions(rt, opts)
			require.Equal(t, &common.LocatorOptions{HasText: tt.want, HasNotText: tt.want}, got)
		})
	}
}
