package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/cloudapi"
	"go.k6.io/k6/v2/internal/cmd/tests"
)

func TestMaskToken(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		token    string
		expected string
	}{
		{
			name:     "empty string returns empty string",
			token:    "",
			expected: "",
		},
		{
			name:     "single character is fully masked",
			token:    "a",
			expected: "*",
		},
		{
			name:     "four characters are fully masked",
			token:    "abcd",
			expected: "****",
		},
		{
			name:     "eleven characters are fully masked",
			token:    "abcdefghijk",
			expected: "***********",
		},
		{
			name:     "twelve characters masks the middle four",
			token:    "abcdefghijkl",
			expected: "abcd****ijkl",
		},
		{
			name:     "long token masks all but first and last four",
			token:    "tok_abcdefghijklmnopqrstuvwxyz1234",
			expected: "tok_" + strings.Repeat("*", 26) + "1234",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := maskToken(tc.token)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestPrintConfigTokenOutput(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		token    string
		expected string
	}{
		{
			name:     "unset token shows not set placeholder",
			token:    "",
			expected: "<not set>",
		},
		{
			name:     "token masked",
			token:    "abcdefghijkl",
			expected: "abcd****ijkl",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts := tests.NewGlobalTestState(t)
			conf := cloudapi.Config{}
			if tc.token != "" {
				conf.Token = null.StringFrom(tc.token)
			}

			printConfig(ts.GlobalState, conf)
			assert.Contains(t, ts.Stdout.String(), "  token: "+tc.expected)
		})
	}
}

func TestNormalizeStackURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"slug", "my-team", "https://my-team.grafana.net"},
		{"slug with .grafana.net suffix", "my-team.grafana.net", "https://my-team.grafana.net"},
		{"full https url", "https://my-team.grafana.net", "https://my-team.grafana.net"},
		{"full http url", "http://my-team.grafana.net", "http://my-team.grafana.net"},
		// https://github.com/grafana/k6/issues/5894
		{"https url with trailing slash", "https://my-team.grafana.net/", "https://my-team.grafana.net"},
		{"https url with several trailing slashes", "https://my-team.grafana.net///", "https://my-team.grafana.net"},
		{"slug with trailing slash", "my-team/", "https://my-team.grafana.net"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, normalizeStackURL(tc.in))
		})
	}
}
