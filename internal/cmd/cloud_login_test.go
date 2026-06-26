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

func TestNormalizeStackURL(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "full URL is kept as is",
			input:    "https://my-team.grafana.net",
			expected: "https://my-team.grafana.net",
		},
		{
			name:     "full URL with trailing slash is trimmed",
			input:    "https://my-team.grafana.net/",
			expected: "https://my-team.grafana.net",
		},
		{
			name:     "slug is expanded to a full URL",
			input:    "my-team",
			expected: "https://my-team.grafana.net",
		},
		{
			name:     "slug with trailing slash is trimmed then expanded",
			input:    "my-team/",
			expected: "https://my-team.grafana.net",
		},
		{
			name:     "grafana.net suffix is stripped from a slug",
			input:    "my-team.grafana.net",
			expected: "https://my-team.grafana.net",
		},
		{
			name:     "grafana.net suffix with trailing slash is stripped from a slug",
			input:    "my-team.grafana.net/",
			expected: "https://my-team.grafana.net",
		},
		{
			name:     "localhost URL is kept as is",
			input:    "http://localhost:8080",
			expected: "http://localhost:8080",
		},
		{
			name:     "localhost URL with trailing slash is trimmed",
			input:    "http://localhost:8080/",
			expected: "http://localhost:8080",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.expected, normalizeStackURL(tc.input))
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
