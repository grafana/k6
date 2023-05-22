package browser

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/xk6-browser/env"
)

func TestIsRemoteBrowser(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		envLookup      env.LookupFunc
		expIsRemote    bool
		expValidWSURLs []string
	}{
		{
			name: "browser is not remote",
			envLookup: func(key string) (string, bool) {
				return "", false
			},
			expIsRemote: false,
		},
		{
			name: "single WS URL",
			envLookup: func(key string) (string, bool) {
				return "WS_URL", true
			},
			expIsRemote:    true,
			expValidWSURLs: []string{"WS_URL"},
		},
		{
			name: "multiple WS URL",
			envLookup: func(key string) (string, bool) {
				return "WS_URL_1,WS_URL_2,WS_URL_3", true
			},
			expIsRemote:    true,
			expValidWSURLs: []string{"WS_URL_1", "WS_URL_2", "WS_URL_3"},
		},
		{
			name: "ending comma is handled",
			envLookup: func(key string) (string, bool) {
				return "WS_URL_1,WS_URL_2,", true
			},
			expIsRemote:    true,
			expValidWSURLs: []string{"WS_URL_1", "WS_URL_2"},
		},
		{
			name: "void string does not panic",
			envLookup: func(key string) (string, bool) {
				return "", true
			},
			expIsRemote:    true,
			expValidWSURLs: []string{""},
		},
		{
			name: "comma does not panic",
			envLookup: func(key string) (string, bool) {
				return ",", true
			},
			expIsRemote:    true,
			expValidWSURLs: []string{""},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rr := newRemoteRegistry(tc.envLookup)
			wsURL, isRemote := rr.IsRemoteBrowser()

			require.Equal(t, tc.expIsRemote, isRemote)
			if isRemote {
				require.Contains(t, tc.expValidWSURLs, wsURL)
			}
		})
	}
}
