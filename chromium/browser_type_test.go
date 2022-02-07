/*
 *
 * xk6-browser - a browser automation extension for k6
 * Copyright (C) 2021 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package chromium

import (
	"net"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	k6lib "go.k6.io/k6/lib"

	"github.com/grafana/xk6-browser/common"
)

func TestBrowserTypeFlags(t *testing.T) {
	t.Parallel()

	host, err := k6lib.NewHostAddress(net.ParseIP("127.0.0.1"), "8000")
	require.NoError(t, err)

	testCases := []struct {
		flag                      string
		changeOpts                *common.LaunchOptions
		changeK6Opts              *k6lib.Options
		expInitVal, expChangedVal interface{}
		pre                       func(t *testing.T)
		post                      func(t *testing.T, flags map[string]interface{})
	}{
		{
			flag:          "auto-open-devtools-for-tabs",
			expInitVal:    false,
			changeOpts:    &common.LaunchOptions{Devtools: true},
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
			post: func(t *testing.T, flags map[string]interface{}) {
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
				Hosts: map[string]*k6lib.HostAddress{
					"test.k6.io":         host,
					"httpbin.test.k6.io": host,
				},
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
			flag:       "enable-use-zoom-for-dsf",
			expInitVal: false,
			pre: func(t *testing.T) {
				if runtime.GOOS != "darwin" {
					t.Skip()
				}
			},
		},
		{
			flag:          "headless",
			expInitVal:    false,
			changeOpts:    &common.LaunchOptions{Headless: true},
			expChangedVal: true,
			post: func(t *testing.T, flags map[string]interface{}) {
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
			if tc.pre != nil {
				tc.pre(t)
			}

			var (
				bt    BrowserType
				flags = bt.flags(&common.LaunchOptions{}, nil)
			)

			if tc.expInitVal != nil {
				require.Contains(t, flags, tc.flag)
				assert.Equal(t, tc.expInitVal, flags[tc.flag])
			} else {
				require.NotContains(t, flags, tc.flag)
			}

			if tc.changeOpts != nil || tc.changeK6Opts != nil {
				flags = bt.flags(tc.changeOpts, tc.changeK6Opts)
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
