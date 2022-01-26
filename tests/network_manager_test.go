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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	k6types "go.k6.io/k6/lib/types"
)

func TestDataURLSkipRequest(t *testing.T) {
	tb := newTestBrowser(t, withLogCache())
	p := tb.NewPage(nil)

	p.Goto("data:text/html,hello", nil)

	assert.True(t, tb.logCache.contains("skipped request handling of data URL"))
}

func TestBlockHostnames(t *testing.T) {
	tb := newTestBrowser(t, withHTTPServer(), withLogCache())

	blocked, err := k6types.NewNullHostnameTrie([]string{"*.test"})
	require.NoError(t, err)
	tb.state.Options.BlockedHostnames = blocked

	p := tb.NewPage(nil)
	res := p.Goto("http://host.test/", nil)
	assert.Nil(t, res)

	assert.True(t, tb.logCache.contains("was interrupted: hostname host.test is in a blocked pattern"))

	// Ensure other requests go through
	resp := p.Goto(tb.URL("/get"), nil)
	assert.NotNil(t, resp)
}
