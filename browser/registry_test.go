package browser

import (
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		expErr         error
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
		{
			name:           "read scenarios without any ws urls",
			expIsRemote:    false,
			expValidWSURLs: []string{""},
			envVarName:     "K6_INSTANCE_SCENARIOS",
			envVarValue:    `[{"id": "one","browsers": [{}]}]`,
		},
		{
			name:           "read scenarios without any browser objects",
			expIsRemote:    false,
			expValidWSURLs: []string{""},
			envVarName:     "K6_INSTANCE_SCENARIOS",
			envVarValue:    `[{"id": "one"}]`,
		},
		{
			name:        "read empty scenarios",
			expErr:      errors.New("parsing K6_INSTANCE_SCENARIOS: unexpected end of JSON input"),
			envVarName:  "K6_INSTANCE_SCENARIOS",
			envVarValue: ``,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.envVarName, tc.envVarValue)

			rr, err := newRemoteRegistry(os.LookupEnv)
			if tc.expErr != nil {
				assert.Error(t, tc.expErr, err)
				return
			}
			assert.NoError(t, err)

			wsURL, isRemote := rr.isRemoteBrowser()

			require.Equal(t, tc.expIsRemote, isRemote)
			if isRemote {
				require.Contains(t, tc.expValidWSURLs, wsURL)
			}
		})
	}

	t.Run("K6_INSTANCE_SCENARIOS should override K6_BROWSER_WS_URL", func(t *testing.T) {
		t.Setenv("K6_BROWSER_WS_URL", "WS_URL_1")
		t.Setenv("K6_INSTANCE_SCENARIOS", `[{"id": "one","browsers": [{ "handle": "WS_URL_2" }]}]`)

		rr, err := newRemoteRegistry(os.LookupEnv)
		assert.NoError(t, err)

		wsURL, isRemote := rr.isRemoteBrowser()

		require.Equal(t, true, isRemote)
		require.Equal(t, "WS_URL_2", wsURL)
	})
}
