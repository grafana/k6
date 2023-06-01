package browser

import (
	"errors"
	"os"
	"strconv"
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
		name                    string
		envVarName, envVarValue string
		expIsRemote             bool
		expValidWSURLs          []string
		expErr                  error
	}{
		{
			name:        "browser is not remote",
			envVarName:  "FOO",
			envVarValue: "BAR",
			expIsRemote: false,
		},
		{
			name:           "single WS URL",
			envVarName:     "K6_BROWSER_WS_URL",
			envVarValue:    "WS_URL",
			expIsRemote:    true,
			expValidWSURLs: []string{"WS_URL"},
		},
		{
			name:           "multiple WS URL",
			envVarName:     "K6_BROWSER_WS_URL",
			envVarValue:    "WS_URL_1,WS_URL_2,WS_URL_3",
			expIsRemote:    true,
			expValidWSURLs: []string{"WS_URL_1", "WS_URL_2", "WS_URL_3"},
		},
		{
			name:           "ending comma is handled",
			envVarName:     "K6_BROWSER_WS_URL",
			envVarValue:    "WS_URL_1,WS_URL_2,",
			expIsRemote:    true,
			expValidWSURLs: []string{"WS_URL_1", "WS_URL_2"},
		},
		{
			name:           "void string does not panic",
			envVarName:     "K6_BROWSER_WS_URL",
			envVarValue:    "",
			expIsRemote:    true,
			expValidWSURLs: []string{""},
		},
		{
			name:           "comma does not panic",
			envVarName:     "K6_BROWSER_WS_URL",
			envVarValue:    ",",
			expIsRemote:    true,
			expValidWSURLs: []string{""},
		},
		{
			name:           "read a single scenario with a single ws url",
			envVarName:     "K6_INSTANCE_SCENARIOS",
			envVarValue:    `[{"id": "one","browsers": [{ "handle": "WS_URL_1" }]}]`,
			expIsRemote:    true,
			expValidWSURLs: []string{"WS_URL_1"},
		},
		{
			name:           "read a single scenario with a two ws urls",
			envVarName:     "K6_INSTANCE_SCENARIOS",
			envVarValue:    `[{"id": "one","browsers": [{"handle": "WS_URL_1"}, {"handle": "WS_URL_2"}]}]`,
			expIsRemote:    true,
			expValidWSURLs: []string{"WS_URL_1", "WS_URL_2"},
		},
		{
			name:       "read two scenarios with multiple ws urls",
			envVarName: "K6_INSTANCE_SCENARIOS",
			envVarValue: `[
				{"id": "one","browsers": [{"handle": "WS_URL_1"}, {"handle": "WS_URL_2"}]},
				{"id": "two","browsers": [{"handle": "WS_URL_3"}, {"handle": "WS_URL_4"}]}
			]`,
			expIsRemote:    true,
			expValidWSURLs: []string{"WS_URL_1", "WS_URL_2", "WS_URL_3", "WS_URL_4"},
		},
		{
			name:           "read scenarios without any ws urls",
			envVarName:     "K6_INSTANCE_SCENARIOS",
			envVarValue:    `[{"id": "one","browsers": [{}]}]`,
			expIsRemote:    false,
			expValidWSURLs: []string{""},
		},
		{
			name:           "read scenarios without any browser objects",
			envVarName:     "K6_INSTANCE_SCENARIOS",
			envVarValue:    `[{"id": "one"}]`,
			expIsRemote:    false,
			expValidWSURLs: []string{""},
		},
		{
			name:        "read empty scenarios",
			envVarName:  "K6_INSTANCE_SCENARIOS",
			envVarValue: ``,
			expErr:      errors.New("parsing K6_INSTANCE_SCENARIOS: unexpected end of JSON input"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// the real environment  variable we receive needs quoting.
			// this makes it as close to the real implementation as possible.
			v := tc.envVarValue
			if tc.envVarName == "K6_INSTANCE_SCENARIOS" {
				v = strconv.Quote(v)
			}
			t.Setenv(tc.envVarName, v)

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
		t.Setenv("K6_INSTANCE_SCENARIOS", strconv.Quote(`[{"id": "one","browsers": [{ "handle": "WS_URL_2" }]}]`))

		rr, err := newRemoteRegistry(os.LookupEnv)
		assert.NoError(t, err)

		wsURL, isRemote := rr.isRemoteBrowser()

		require.Equal(t, true, isRemote)
		require.Equal(t, "WS_URL_2", wsURL)
	})
}
