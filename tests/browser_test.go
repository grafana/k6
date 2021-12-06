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
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBrowserNewPage(t *testing.T) {
	t.Parallel()

	b := TestBrowser(t)
	p := b.NewPage(nil)
	l := len(b.Contexts())
	assert.Equal(t, 1, l, "expected there to be 1 browser context, but found %d", l)

	p2 := b.NewPage(nil)
	l = len(b.Contexts())
	assert.Equal(t, 2, l, "expected there to be 2 browser context, but found %d", l)

	p.Close(nil)
	l = len(b.Contexts())
	assert.Equal(t, 1, l, "expected there to be 1 browser context after first page close, but found %d", l)
	p2.Close(nil)
	l = len(b.Contexts())
	assert.Equal(t, 0, l, "expected there to be 0 browser context after second page close, but found %d", l)
}

// This only works for Chrome!
func TestBrowserVersion(t *testing.T) {
	t.Parallel()

	const re = `^\d+\.\d+\.\d+\.\d+$`
	r, _ := regexp.Compile(re)
	ver := TestBrowser(t).Version()
	assert.Regexp(t, r, ver, "expected browser version to match regex %q, but found %q", re, ver)
}

// This only works for Chrome!
// TODO: Improve this test, see:
// https://github.com/grafana/xk6-browser/pull/51#discussion_r742696736
func TestBrowserUserAgent(t *testing.T) {
	t.Parallel()

	b := TestBrowser(t)

	// testBrowserVersion() tests the version already
	// just look for "Headless" in UserAgent
	ua := b.UserAgent()
	if prefix := "Mozilla/5.0"; !strings.HasPrefix(ua, prefix) {
		t.Errorf("UserAgent should start with %q, but got: %q", prefix, ua)
	}
	assert.Contains(t, ua, "Headless")
}
