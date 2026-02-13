package browser

import (
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext/k6test"
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

func TestParseSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    *common.Size
		wantErr string
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  &common.Size{Width: 0, Height: 0},
		},
		{
			name:  "all_options",
			input: `({width: 1920, height: 1080})`,
			want:  &common.Size{Width: 1920, Height: 1080},
		},
		{
			name:  "partial_width_only",
			input: `({width: 1920})`,
			want:  &common.Size{Width: 1920, Height: 0},
		},
		{
			name:  "float_width",
			input: `({width: 1920.5, height: 1080})`,
			want:  &common.Size{Width: 1920.5, Height: 1080},
		},
		{
			name:  "float_height",
			input: `({width: 1920, height: 1080.5})`,
			want:  &common.Size{Width: 1920, Height: 1080.5},
		},
		{
			name:  "null_width",
			input: `({width: null, height: 1080})`,
			want:  &common.Size{Width: 0, Height: 1080},
		},
		{
			name:  "undefined_width",
			input: `({width: undefined, height: 1080})`,
			want:  &common.Size{Width: 0, Height: 1080},
		},
		{
			name:    "invalid_width_string",
			input:   `({width: "1920", height: 1080})`,
			wantErr: "width must be a number",
		},
		{
			name:    "invalid_height_string",
			input:   `({width: 1920, height: "1080"})`,
			wantErr: "height must be a number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseSize(vu.Runtime(), v)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, opts)
		})
	}
}

func TestParsePageEmulateMediaOptions(t *testing.T) {
	t.Parallel()

	newDefaults := func() *common.PageEmulateMediaOptions {
		return &common.PageEmulateMediaOptions{
			ColorScheme:   common.ColorSchemeLight,
			Media:         common.MediaTypeScreen,
			ReducedMotion: common.ReducedMotionNoPreference,
		}
	}

	tests := []struct {
		name  string
		input string
		want  *common.PageEmulateMediaOptions
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  newDefaults(),
		},
		{
			name:  "all_options",
			input: `({colorScheme: "dark", media: "print", reducedMotion: "reduce"})`,
			want: &common.PageEmulateMediaOptions{
				ColorScheme:   "dark",
				Media:         "print",
				ReducedMotion: "reduce",
			},
		},
		{
			name:  "partial_option",
			input: `({media: "print"})`,
			want: &common.PageEmulateMediaOptions{
				ColorScheme:   common.ColorSchemeLight,
				Media:         "print",
				ReducedMotion: common.ReducedMotionNoPreference,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parsePageEmulateMediaOptions(vu.Runtime(), v, newDefaults())
			require.NoError(t, err)
			assert.Equal(t, tt.want, opts)
		})
	}
}
