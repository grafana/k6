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

	t.Run("devtools", func(t *testing.T) {
		t.Parallel()

		const devToolsFlag = "auto-open-devtools-for-tabs"

		var (
			bt    BrowserType
			lopts = &common.LaunchOptions{}
		)

		flags := bt.flags(lopts)
		require.Contains(t, flags, devToolsFlag)

		_, ok := flags[devToolsFlag].(bool)
		require.Truef(t, ok, "%q should be a bool", devToolsFlag)

		lopts.Devtools = false
		assert.Falsef(t,
			flags[devToolsFlag].(bool),
			"%q should also be false if launch options Devtools is false", devToolsFlag,
		)

		flags = bt.flags(&common.LaunchOptions{
			Devtools: true,
		})
		assert.Truef(t,
			flags[devToolsFlag].(bool),
			"%q should be true when launch options Devtools is true", devToolsFlag,
		)
	})

	t.Run("headless", func(t *testing.T) {
		t.Parallel()

		const headlessFlag = "headless"

		var (
			bt    BrowserType
			lopts = &common.LaunchOptions{}
		)

		flags := bt.flags(lopts)
		require.Contains(t, flags, headlessFlag)

		_, ok := flags[headlessFlag].(bool)
		require.Truef(t, ok, "%q should be a bool", headlessFlag)

		lopts.Headless = false
		assert.Falsef(t,
			flags[headlessFlag].(bool),
			"%q should also be false if launch options Headless is false", headlessFlag,
		)

		flags = bt.flags(&common.LaunchOptions{
			Headless: true,
		})
		assert.Truef(t,
			flags[headlessFlag].(bool),
			"%q should be true when launch options Headless is true", headlessFlag,
		)
	})

	t.Run("headless/enabled", func(t *testing.T) {
		t.Parallel()

		var (
			before, after int
			bt            BrowserType
			lopts         = &common.LaunchOptions{}
		)

		lopts.Headless = false
		before = len(bt.flags(lopts))

		lopts.Headless = true
		after = len(bt.flags(lopts))

		assert.NotEqual(t, before, after, "enabling headless mode did not add flags")
	})

	t.Run("darwin", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS != "darwin" {
			t.Skip()
		}

		const zoomFlag = "enable-use-zoom-for-dsf"

		var bt BrowserType
		f := bt.flags(&common.LaunchOptions{})

		assert.Containsf(t, f, zoomFlag, "darwin should disable %q", zoomFlag)

		flag, ok := f[zoomFlag].(bool)
		require.Truef(t, ok, "%q should be a bool", zoomFlag)
		assert.False(t, flag, zoomFlag)
	})
}
