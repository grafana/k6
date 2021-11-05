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

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTimeoutSettings(t *testing.T) {
	t.Run("TimeoutSettings.NewTimeoutSettings", func(t *testing.T) {
		t.Run("should work", func(t *testing.T) { testTimeoutSettingsNewTimeoutSettings(t) })
		t.Run("should work with parent", func(t *testing.T) { testTimeoutSettingsNewTimeoutSettingsWithParent(t) })
	})
	t.Run("TimeoutSettings.setDefaultTimeout", func(t *testing.T) {
		t.Run("should work", func(t *testing.T) { testTimeoutSettingsSetDefaultTimeout(t) })
	})
	t.Run("TimeoutSettings.setDefaultNavigationTimeout", func(t *testing.T) {
		t.Run("should work", func(t *testing.T) { testTimeoutSettingsSetDefaultNavigationTimeout(t) })
	})
	t.Run("TimeoutSettings.navigationTimeout", func(t *testing.T) {
		t.Run("should work", func(t *testing.T) { testTimeoutSettingsNavigationTimeout(t) })
		t.Run("should work with parent", func(t *testing.T) { testTimeoutSettingsNavigationTimeoutWithParent(t) })
	})
	t.Run("TimeoutSettings.timeout", func(t *testing.T) {
		t.Run("should work", func(t *testing.T) { testTimeoutSettingsTimeout(t) })
		t.Run("should work with parent", func(t *testing.T) { testTimeoutSettingsTimeoutWithParent(t) })
	})
}

func testTimeoutSettingsNewTimeoutSettings(t *testing.T) {
	ts := NewTimeoutSettings(nil)
	assert.Nil(t, ts.parent)
	assert.Nil(t, ts.defaultTimeout)
	assert.Nil(t, ts.defaultNavigationTimeout)
}

func testTimeoutSettingsNewTimeoutSettingsWithParent(t *testing.T) {
	ts := NewTimeoutSettings(nil)
	tsWithParent := NewTimeoutSettings(ts)
	assert.Equal(t, ts, tsWithParent.parent)
	assert.Nil(t, tsWithParent.defaultTimeout)
	assert.Nil(t, tsWithParent.defaultNavigationTimeout)
}

func testTimeoutSettingsSetDefaultTimeout(t *testing.T) {
	ts := NewTimeoutSettings(nil)
	ts.setDefaultTimeout(100)
	assert.Equal(t, int64(100), *ts.defaultTimeout)
}

func testTimeoutSettingsSetDefaultNavigationTimeout(t *testing.T) {
	ts := NewTimeoutSettings(nil)
	ts.setDefaultNavigationTimeout(100)
	assert.Equal(t, int64(100), *ts.defaultNavigationTimeout)
}

func testTimeoutSettingsNavigationTimeout(t *testing.T) {
	ts := NewTimeoutSettings(nil)

	// Assert default timeout value is used
	assert.Equal(t, int64(DefaultTimeout.Seconds()), ts.navigationTimeout())

	// Assert custom default timeout is used
	ts.setDefaultNavigationTimeout(100)
	assert.Equal(t, int64(100), ts.navigationTimeout())
}

func testTimeoutSettingsNavigationTimeoutWithParent(t *testing.T) {
	ts := NewTimeoutSettings(nil)
	tsWithParent := NewTimeoutSettings(ts)

	// Assert default timeout value is used
	assert.Equal(t, int64(DefaultTimeout.Seconds()), tsWithParent.navigationTimeout())

	// Assert custom default timeout from parent is used
	ts.setDefaultNavigationTimeout(1000)
	assert.Equal(t, int64(1000), tsWithParent.navigationTimeout())

	// Assert custom default timeout is used (over parent)
	tsWithParent.setDefaultNavigationTimeout(100)
	assert.Equal(t, int64(100), tsWithParent.navigationTimeout())
}

func testTimeoutSettingsTimeout(t *testing.T) {
	ts := NewTimeoutSettings(nil)

	// Assert default timeout value is used
	assert.Equal(t, int64(DefaultTimeout.Seconds()), ts.timeout())

	// Assert custom default timeout is used
	ts.setDefaultTimeout(100)
	assert.Equal(t, int64(100), ts.timeout())
}

func testTimeoutSettingsTimeoutWithParent(t *testing.T) {
	ts := NewTimeoutSettings(nil)
	tsWithParent := NewTimeoutSettings(ts)

	// Assert default timeout value is used
	assert.Equal(t, int64(DefaultTimeout.Seconds()), tsWithParent.timeout())

	// Assert custom default timeout from parent is used
	ts.setDefaultTimeout(1000)
	assert.Equal(t, int64(1000), tsWithParent.timeout())

	// Assert custom default timeout is used (over parent)
	tsWithParent.setDefaultTimeout(100)
	assert.Equal(t, int64(100), tsWithParent.timeout())
}
