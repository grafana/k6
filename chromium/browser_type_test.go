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
	"runtime"
	"testing"

	"github.com/grafana/xk6-browser/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBrowserTypeFlags(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		flag                      string
		changeOpts                *common.LaunchOptions
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
				flags = bt.flags(&common.LaunchOptions{})
			)

			if tc.expInitVal != nil {
				require.Contains(t, flags, tc.flag)
				assert.Equal(t, tc.expInitVal, flags[tc.flag])
			}

			if tc.changeOpts != nil {
				flags = bt.flags(tc.changeOpts)
				assert.Equal(t, tc.expChangedVal, flags[tc.flag])
			}

			if tc.post != nil {
				tc.post(t, flags)
			}
		})
	}
}
