package chromium

import (
	"net"
	"testing"

	"github.com/grafana/xk6-browser/common"

	k6lib "go.k6.io/k6/lib"
	"go.k6.io/k6/lib/types"
	k6types "go.k6.io/k6/lib/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBrowserTypePrepareFlags(t *testing.T) {
	t.Parallel()

	// to be used by the tests below
	host, err := k6types.NewHost(net.ParseIP("127.0.0.1"), "8000")
	require.NoError(t, err, "failed to set up test host")
	hosts, err := k6types.NewHosts(map[string]k6types.Host{
		"test.k6.io":         *host,
		"httpbin.test.k6.io": *host,
	})
	require.NoError(t, err, "failed to set up test hosts")

	testCases := []struct {
		flag                      string
		changeOpts                *common.LaunchOptions
		changeK6Opts              *k6lib.Options
		expInitVal, expChangedVal any
		post                      func(t *testing.T, flags map[string]any)
	}{
		{
			flag:       "hide-scrollbars",
			changeOpts: &common.LaunchOptions{IgnoreDefaultArgs: []string{"hide-scrollbars"}, Headless: true},
		},
		{
			flag:          "hide-scrollbars",
			changeOpts:    &common.LaunchOptions{Headless: true},
			expChangedVal: true,
		},
		{
			flag:          "browser-arg",
			expInitVal:    nil,
			changeOpts:    &common.LaunchOptions{Args: []string{"browser-arg=value"}},
			expChangedVal: "value",
		},
		{
			flag:          "browser-arg-flag",
			expInitVal:    nil,
			changeOpts:    &common.LaunchOptions{Args: []string{"browser-arg-flag"}},
			expChangedVal: "",
		},
		{
			flag:       "browser-arg-trim-double-quote",
			expInitVal: nil,
			changeOpts: &common.LaunchOptions{Args: []string{
				`   browser-arg-trim-double-quote =  "value  "  `,
			}},
			expChangedVal: "value  ",
		},
		{
			flag:       "browser-arg-trim-single-quote",
			expInitVal: nil,
			changeOpts: &common.LaunchOptions{Args: []string{
				`   browser-arg-trim-single-quote=' value '`,
			}},
			expChangedVal: " value ",
		},
		{
			flag:       "browser-args",
			expInitVal: nil,
			changeOpts: &common.LaunchOptions{Args: []string{
				"browser-arg1='value1", "browser-arg2=''value2''", "browser-flag",
			}},
			post: func(t *testing.T, flags map[string]any) {
				t.Helper()

				assert.Equal(t, "'value1", flags["browser-arg1"])
				assert.Equal(t, "'value2'", flags["browser-arg2"])
				assert.Equal(t, "", flags["browser-flag"])
			},
		},
		{
			flag:       "host-resolver-rules",
			expInitVal: nil,
			changeOpts: &common.LaunchOptions{Args: []string{
				`host-resolver-rules="MAP * www.example.com, EXCLUDE *.youtube.*"`,
			}},
			changeK6Opts: &k6lib.Options{
				Hosts: types.NullHosts{Trie: hosts, Valid: true},
			},
			expChangedVal: "MAP * www.example.com, EXCLUDE *.youtube.*," +
				"MAP httpbin.test.k6.io 127.0.0.1:8000,MAP test.k6.io 127.0.0.1:8000",
		},
		{
			flag:          "host-resolver-rules",
			expInitVal:    nil,
			changeOpts:    &common.LaunchOptions{},
			changeK6Opts:  &k6lib.Options{},
			expChangedVal: nil,
		},
		{
			flag:          "headless",
			expInitVal:    false,
			changeOpts:    &common.LaunchOptions{Headless: true},
			expChangedVal: true,
			post: func(t *testing.T, flags map[string]any) {
				t.Helper()

				extraFlags := []string{"hide-scrollbars", "mute-audio", "blink-settings"}
				for _, f := range extraFlags {
					assert.Contains(t, flags, f)
				}
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.flag, func(t *testing.T) {
			t.Parallel()

			flags, err := prepareFlags(&common.LaunchOptions{}, nil)
			require.NoError(t, err, "failed to prepare flags")

			if tc.expInitVal != nil {
				require.Contains(t, flags, tc.flag)
				assert.Equal(t, tc.expInitVal, flags[tc.flag])
			} else {
				require.NotContains(t, flags, tc.flag)
			}

			if tc.changeOpts != nil || tc.changeK6Opts != nil {
				flags, err = prepareFlags(tc.changeOpts, tc.changeK6Opts)
				require.NoError(t, err, "failed to prepare flags")
				if tc.expChangedVal != nil {
					assert.Equal(t, tc.expChangedVal, flags[tc.flag])
				} else {
					assert.NotContains(t, flags, tc.flag)
				}
			}

			if tc.post != nil {
				tc.post(t, flags)
			}
		})
	}
}
