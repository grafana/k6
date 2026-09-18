package log

import (
	"io"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newParentLogger() (*logrus.Logger, *logrusHook) { //nolint:forbidigo
	parent := logrus.New()
	parent.SetLevel(logrus.InfoLevel)
	parent.SetOutput(io.Discard)
	hook := &logrusHook{levels: logrus.AllLevels}
	parent.AddHook(hook)
	return parent, hook
}

type logrusHook struct {
	levels  []logrus.Level
	entries []string
}

func (h *logrusHook) Levels() []logrus.Level { return h.levels }

func (h *logrusHook) Fire(e *logrus.Entry) error {
	h.entries = append(h.entries, e.Message)
	return nil
}

func TestSetLevelDoesNotMutateParentLogger(t *testing.T) {
	t.Parallel()

	parent, hook := newParentLogger()
	bl := New(parent, "iter")
	require.NoError(t, bl.SetLevel("error"))

	assert.Equal(t, logrus.InfoLevel, parent.GetLevel())

	parent.Info("from-k6")
	bl.Infof("browser", "from-browser-info")
	bl.Errorf("browser", "from-browser-error")

	assert.Contains(t, hook.entries, "from-k6")
	assert.Contains(t, hook.entries, "from-browser-error")
	assert.NotContains(t, hook.entries, "from-browser-info")
}

func TestSetLevelCanBeMoreVerboseThanParent(t *testing.T) {
	t.Parallel()

	parent, hook := newParentLogger()
	bl := New(parent, "iter")
	require.NoError(t, bl.SetLevel("debug"))

	assert.Equal(t, logrus.InfoLevel, parent.GetLevel())
	bl.Debugf("browser", "from-browser-debug")
	assert.Contains(t, hook.entries, "from-browser-debug")
}

func TestGetLevelFollowsParentUntilOverride(t *testing.T) {
	t.Parallel()

	parent, _ := newParentLogger()
	bl := New(parent, "")
	assert.Equal(t, logrus.InfoLevel, bl.GetLevel())

	parent.SetLevel(logrus.DebugLevel)
	assert.Equal(t, logrus.DebugLevel, bl.GetLevel())
	assert.True(t, bl.DebugMode())

	require.NoError(t, bl.SetLevel("error"))
	assert.Equal(t, logrus.ErrorLevel, bl.GetLevel())
	assert.Equal(t, logrus.DebugLevel, parent.GetLevel())
	assert.False(t, bl.DebugMode())
}
