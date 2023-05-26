package browser

import (
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/xk6-browser/env"
)

func TestPidRegistry(t *testing.T) {
	p := &pidRegistry{}

	var wg sync.WaitGroup
	expected := []int{}
	iteration := 100
	wg.Add(iteration)
	for i := 0; i < iteration; i++ {
		go func(i int) {
			p.registerPid(i)
			wg.Done()
		}(i)
		expected = append(expected, i)
	}

	wg.Wait()

	got := p.Pids()

	assert.ElementsMatch(t, expected, got)
}

func TestIsRemoteBrowser(t *testing.T) {
	testCases := []struct {
		name           string
		expIsRemote    bool
		expValidWSURLs []string
		envVarName     string
		envVarValue    string
	}{
		{
			name:        "browser is not remote",
			expIsRemote: false,
			envVarName:  "FOO",
			envVarValue: "BAR",
		},
		{
			name:           "single WS URL",
			expIsRemote:    true,
			expValidWSURLs: []string{"WS_URL"},
			envVarName:     "K6_BROWSER_WS_URL",
			envVarValue:    "WS_URL",
		},
		{
			name:           "multiple WS URL",
			expIsRemote:    true,
			expValidWSURLs: []string{"WS_URL_1", "WS_URL_2", "WS_URL_3"},
			envVarName:     "K6_BROWSER_WS_URL",
			envVarValue:    "WS_URL_1,WS_URL_2,WS_URL_3",
		},
		{
			name:           "ending comma is handled",
			expIsRemote:    true,
			expValidWSURLs: []string{"WS_URL_1", "WS_URL_2"},
			envVarName:     "K6_BROWSER_WS_URL",
			envVarValue:    "WS_URL_1,WS_URL_2,",
		},
		{
			name:           "void string does not panic",
			expIsRemote:    true,
			expValidWSURLs: []string{""},
			envVarName:     "K6_BROWSER_WS_URL",
			envVarValue:    "",
		},
		{
			name:           "comma does not panic",
			expIsRemote:    true,
			expValidWSURLs: []string{""},
			envVarName:     "K6_BROWSER_WS_URL",
			envVarValue:    ",",
		},
		{
			name:           "read a single scenario with a single ws url",
			expIsRemote:    true,
			expValidWSURLs: []string{"WS_URL_1"},
			envVarName:     "K6_INSTANCE_SCENARIOS",
			envVarValue:    `[{"id": "one","browsers": [{ "handle": "WS_URL_1" }]}]`,
		},
		{
			name:           "read a single scenario with a two ws urls",
			expIsRemote:    true,
			expValidWSURLs: []string{"WS_URL_1", "WS_URL_2"},
			envVarName:     "K6_INSTANCE_SCENARIOS",
			envVarValue:    `[{"id": "one","browsers": [{"handle": "WS_URL_1"}, {"handle": "WS_URL_2"}]}]`,
		},
		{
			name:           "read two scenarios with multiple ws urls",
			expIsRemote:    true,
			expValidWSURLs: []string{"WS_URL_1", "WS_URL_2", "WS_URL_3", "WS_URL_4"},
			envVarName:     "K6_INSTANCE_SCENARIOS",
			envVarValue: `[{"id": "one","browsers": [{"handle": "WS_URL_1"}, {"handle": "WS_URL_2"}]},
			{"id": "two","browsers": [{"handle": "WS_URL_3"}, {"handle": "WS_URL_4"}]}]`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.envVarName, tc.envVarValue)
			rr := newRemoteRegistry(os.LookupEnv)
			wsURL, isRemote := rr.isRemoteBrowser()

			require.Equal(t, tc.expIsRemote, isRemote)
			if isRemote {
				require.Contains(t, tc.expValidWSURLs, wsURL)
			}
		})
	}
}

func TestCheckForScenarios(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		envLookup      env.LookupFunc
		expIsRemote    bool
		expValidWSURLs []string
	}{
		{
			name: "multiple scenarios",
			envLookup: func(key string) (string, bool) {
				json := `[
					{
						"id": "one",
						"browsers": [
							{ "handle": "ws://1..." },
							{ "handle": "ws://2..." }
						]
					},
					{
						"id": "two",
						"browsers": [
							{ "handle": "ws://3..." }
						]
					}
				]`

				return json, true
			},
			expIsRemote:    true,
			expValidWSURLs: []string{"ws://1...", "ws://2...", "ws://3..."},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			isRemote, wsURLs, err := checkForScenarios(tc.envLookup)
			assert.NoError(t, err)

			require.Equal(t, tc.expIsRemote, isRemote)
			if isRemote {
				require.Equal(t, tc.expValidWSURLs, wsURLs)
			}
		})
	}
}
