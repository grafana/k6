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

package tests

import (
	"testing"

	"github.com/grafana/xk6-browser/testutils"
	"github.com/grafana/xk6-browser/testutils/browsertest"
	"github.com/stretchr/testify/assert"
)

func TestDataURLSkipRequest(t *testing.T) {
	t.Parallel()
	bt := browsertest.NewBrowserTest(t)
	p := bt.Browser.NewPage(nil)
	t.Cleanup(func() {
		p.Close(nil)
		bt.Browser.Close()
	})

	lc := testutils.AttachLogCache(bt.State.Logger)

	p.Goto("data:text/html,hello", nil)

	assert.True(t, lc.Contains("skipped request handling of data URL"))
}
