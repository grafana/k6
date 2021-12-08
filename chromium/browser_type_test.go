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
)

func TestBrowserTypeFlags(t *testing.T) {
	t.Parallel()

	t.Run("devtools", func(t *testing.T) {
		t.Parallel()

		const devToolsFlag = "auto-open-devtools-for-tabs"

		var bt BrowserType

		flags := bt.flags(&common.LaunchOptions{
			Devtools: false,
		})
		flagi, ok := flags[devToolsFlag]

		if !ok {
			t.Fatalf("%q is missing", devToolsFlag)
		}
		flag, ok := flagi.(bool)
		if !ok {
			t.Fatalf("%q should be a bool", devToolsFlag)
		}
		if flag {
			t.Fatalf("%q should be false when launch options Devtools is false, got %t", devToolsFlag, flag)
		}

		flags = bt.flags(&common.LaunchOptions{
			Devtools: true,
		})
		flag = flags[devToolsFlag].(bool)
		if !flag {
			t.Fatalf("%q should be true when launch options Devtools is true, got %t", devToolsFlag, flag)
		}
	})

	t.Run("headless", func(t *testing.T) {
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

		if before == after {
			t.Errorf("enabling headless mode did not add flags")
		}
	})

	t.Run("darwin", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS != "darwin" {
			t.Skip()
		}

		var bt BrowserType
		f := bt.flags(&common.LaunchOptions{})

		if _, ok := f["enable-use-zoom-for-dsf"]; !ok {
			t.Errorf("darwin should enable 'enable-use-zoom-for-dsf'")
		}
	})
}
