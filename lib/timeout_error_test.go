/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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

package lib

import (
	"strings"
	"testing"
	"time"

	"github.com/loadimpact/k6/lib/consts"
)

func TestTimeoutError(t *testing.T) {
	tests := []struct {
		stage, expectedStrContain string
		d                         time.Duration
	}{
		{consts.SetupFn, "1 seconds", time.Second},
		{consts.TeardownFn, "2 seconds", time.Second * 2},
		{"", "0 seconds", time.Duration(0)},
	}

	for _, tc := range tests {
		te := NewTimeoutError(tc.stage, tc.d)
		if !strings.Contains(te.String(), tc.expectedStrContain) {
			t.Errorf("Expected error contains %s, but got: %s", tc.expectedStrContain, te.String())
		}
	}
}

func TestTimeoutErrorHint(t *testing.T) {
	tests := []struct {
		stage string
		empty bool
	}{
		{consts.SetupFn, false},
		{consts.TeardownFn, false},
		{"not handle", true},
	}

	for _, tc := range tests {
		te := NewTimeoutError(tc.stage, time.Second)
		if tc.empty && te.Hint() != "" {
			t.Errorf("Expected empty hint, got: %s", te.Hint())
		}
		if !tc.empty && te.Hint() == "" {
			t.Errorf("Expected non-empty hint, got empty")
		}
	}
}
