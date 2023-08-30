package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTimeoutSettings(t *testing.T) {
	t.Parallel()

	t.Run("TimeoutSettings.NewTimeoutSettings", func(t *testing.T) {
		t.Parallel()

		t.Run("should work", testTimeoutSettingsNewTimeoutSettings)
		t.Run("should work with parent", testTimeoutSettingsNewTimeoutSettingsWithParent)
	})
	t.Run("TimeoutSettings.setDefaultTimeout", func(t *testing.T) {
		t.Parallel()

		t.Run("should work", testTimeoutSettingsSetDefaultTimeout)
	})
	t.Run("TimeoutSettings.setDefaultNavigationTimeout", func(t *testing.T) {
		t.Parallel()

		t.Run("should work", testTimeoutSettingsSetDefaultNavigationTimeout)
	})
	t.Run("TimeoutSettings.navigationTimeout", func(t *testing.T) {
		t.Parallel()

		t.Run("should work", testTimeoutSettingsNavigationTimeout)
		t.Run("should work with parent", testTimeoutSettingsNavigationTimeoutWithParent)
	})
	t.Run("TimeoutSettings.timeout", func(t *testing.T) {
		t.Parallel()

		t.Run("should work", testTimeoutSettingsTimeout)
		t.Run("should work with parent", testTimeoutSettingsTimeoutWithParent)
	})
}

func testTimeoutSettingsNewTimeoutSettings(t *testing.T) {
	t.Parallel()

	ts := NewTimeoutSettings(nil)
	assert.Nil(t, ts.parent)
	assert.Nil(t, ts.defaultTimeout)
	assert.Nil(t, ts.defaultNavigationTimeout)
}

func testTimeoutSettingsNewTimeoutSettingsWithParent(t *testing.T) {
	t.Parallel()

	ts := NewTimeoutSettings(nil)
	tsWithParent := NewTimeoutSettings(ts)
	assert.Equal(t, ts, tsWithParent.parent)
	assert.Nil(t, tsWithParent.defaultTimeout)
	assert.Nil(t, tsWithParent.defaultNavigationTimeout)
}

func testTimeoutSettingsSetDefaultTimeout(t *testing.T) {
	t.Parallel()

	ts := NewTimeoutSettings(nil)
	ts.setDefaultTimeout(100)
	assert.Equal(t, int64(100), *ts.defaultTimeout)
}

func testTimeoutSettingsSetDefaultNavigationTimeout(t *testing.T) {
	t.Parallel()

	ts := NewTimeoutSettings(nil)
	ts.setDefaultNavigationTimeout(100)
	assert.Equal(t, int64(100), *ts.defaultNavigationTimeout)
}

func testTimeoutSettingsNavigationTimeout(t *testing.T) {
	t.Parallel()

	ts := NewTimeoutSettings(nil)

	// Assert default timeout value is used
	assert.Equal(t, int64(DefaultTimeout.Seconds()), ts.navigationTimeout())

	// Assert custom default timeout is used
	ts.setDefaultNavigationTimeout(100)
	assert.Equal(t, int64(100), ts.navigationTimeout())
}

func testTimeoutSettingsNavigationTimeoutWithParent(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	ts := NewTimeoutSettings(nil)

	// Assert default timeout value is used
	assert.Equal(t, int64(DefaultTimeout.Seconds()), ts.timeout())

	// Assert custom default timeout is used
	ts.setDefaultTimeout(100)
	assert.Equal(t, int64(100), ts.timeout())
}

func testTimeoutSettingsTimeoutWithParent(t *testing.T) {
	t.Parallel()

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
