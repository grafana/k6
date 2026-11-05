package common

import (
	"testing"
	"time"

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
	ts.setDefaultTimeout(time.Duration(100) * time.Millisecond)
	assert.Equal(t, int64(100), ts.defaultTimeout.Milliseconds())
}

func testTimeoutSettingsSetDefaultNavigationTimeout(t *testing.T) {
	t.Parallel()

	ts := NewTimeoutSettings(nil)
	ts.setDefaultNavigationTimeout(time.Duration(100) * time.Millisecond)
	assert.Equal(t, int64(100), ts.defaultNavigationTimeout.Milliseconds())
}

func testTimeoutSettingsNavigationTimeout(t *testing.T) {
	t.Parallel()

	ts := NewTimeoutSettings(nil)

	// Assert default timeout value is used
	assert.Equal(t, DefaultTimeout, ts.navigationTimeout())

	// Assert custom default timeout is used
	ts.setDefaultNavigationTimeout(time.Duration(100) * time.Millisecond)
	assert.Equal(t, int64(100), ts.navigationTimeout().Milliseconds())
}

func testTimeoutSettingsNavigationTimeoutWithParent(t *testing.T) {
	t.Parallel()

	ts := NewTimeoutSettings(nil)
	tsWithParent := NewTimeoutSettings(ts)

	// Assert default timeout value is used
	assert.Equal(t, DefaultTimeout, tsWithParent.navigationTimeout())

	// Assert custom default timeout from parent is used
	ts.setDefaultNavigationTimeout(time.Duration(1000) * time.Millisecond)
	assert.Equal(t, int64(1000), tsWithParent.navigationTimeout().Milliseconds())

	// Assert custom default timeout is used (over parent)
	tsWithParent.setDefaultNavigationTimeout(time.Duration(100) * time.Millisecond)
	assert.Equal(t, int64(100), tsWithParent.navigationTimeout().Milliseconds())
}

func testTimeoutSettingsTimeout(t *testing.T) {
	t.Parallel()

	ts := NewTimeoutSettings(nil)

	// Assert default timeout value is used
	assert.Equal(t, DefaultTimeout, ts.timeout())

	// Assert custom default timeout is used
	ts.setDefaultTimeout(time.Duration(100) * time.Millisecond)
	assert.Equal(t, int64(100), ts.timeout().Milliseconds())
}

func testTimeoutSettingsTimeoutWithParent(t *testing.T) {
	t.Parallel()

	ts := NewTimeoutSettings(nil)
	tsWithParent := NewTimeoutSettings(ts)

	// Assert default timeout value is used
	assert.Equal(t, DefaultTimeout, tsWithParent.timeout())

	// Assert custom default timeout from parent is used
	ts.setDefaultTimeout(time.Duration(1000) * time.Millisecond)
	assert.Equal(t, int64(1000), tsWithParent.timeout().Milliseconds())

	// Assert custom default timeout is used (over parent)
	tsWithParent.setDefaultTimeout(time.Duration(100) * time.Millisecond)
	assert.Equal(t, int64(100), tsWithParent.timeout().Milliseconds())
}

func TestTimeoutSettingsPrecedence(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name           string
		configure      func(parent, child *TimeoutSettings)
		wantTimeout    time.Duration
		wantNavigation time.Duration
	}{
		{
			name: "local default overrides parent navigation timeout",
			configure: func(_, child *TimeoutSettings) {
				child.setDefaultTimeout(3 * time.Second)
			},
			wantTimeout: 3 * time.Second, wantNavigation: 3 * time.Second,
		},
		{
			name: "local navigation overrides local default only for navigation",
			configure: func(_, child *TimeoutSettings) {
				child.setDefaultTimeout(3 * time.Second)
				child.setDefaultNavigationTimeout(4 * time.Second)
			},
			wantTimeout: 3 * time.Second, wantNavigation: 4 * time.Second,
		},
		{
			name: "local navigation preserves inherited default",
			configure: func(_, child *TimeoutSettings) {
				child.setDefaultNavigationTimeout(4 * time.Second)
			},
			wantTimeout: time.Second, wantNavigation: 4 * time.Second,
		},
		{
			name: "zero local default overrides parent timeouts",
			configure: func(_, child *TimeoutSettings) {
				child.setDefaultTimeout(0)
			},
			wantTimeout: 0, wantNavigation: 0,
		},
		{
			name: "zero local navigation overrides nonzero local default",
			configure: func(_, child *TimeoutSettings) {
				child.setDefaultTimeout(3 * time.Second)
				child.setDefaultNavigationTimeout(0)
			},
			wantTimeout: 3 * time.Second, wantNavigation: 0,
		},
		{
			name: "zero parent timeouts are inherited",
			configure: func(parent, _ *TimeoutSettings) {
				parent.setDefaultTimeout(0)
				parent.setDefaultNavigationTimeout(0)
			},
			wantTimeout: 0, wantNavigation: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			parent := NewTimeoutSettings(nil)
			parent.setDefaultTimeout(time.Second)
			parent.setDefaultNavigationTimeout(2 * time.Second)
			child := NewTimeoutSettings(parent)
			tc.configure(parent, child)

			assert.Equal(t, tc.wantTimeout, child.timeout(), "operation timeout")
			assert.Equal(t, tc.wantNavigation, child.navigationTimeout(), "navigation timeout")
		})
	}
}
