package log

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportCallerAddsCallerInfo(t *testing.T) {
	t.Parallel()

	base := logrus.New()
	base.SetLevel(logrus.DebugLevel)
	var buf bytes.Buffer
	base.SetOutput(&buf)
	base.SetFormatter(&logrus.JSONFormatter{})

	l := New(base, "")
	require.NoError(t, l.ReportCaller())

	l.Debugf("category", "hello %s", "world")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))

	caller, ok := entry["caller"].(string)
	require.True(t, ok, "caller field should be present once ReportCaller is enabled")
	assert.Contains(t, caller, "logger.go", "caller should resolve to this package")

	funcName, ok := entry["func"].(string)
	require.True(t, ok, "func field should be present once ReportCaller is enabled")
	assert.Contains(t, funcName, "Logger", "func should name the calling method")
}

// unsupportedFormatter is a logrus.Formatter that isn't a *logrus.TextFormatter
// or *logrus.JSONFormatter, to exercise ReportCaller's default case.
type unsupportedFormatter struct{}

func (unsupportedFormatter) Format(*logrus.Entry) ([]byte, error) {
	return nil, nil
}

func TestReportCallerReturnsErrorForUnsupportedFormatter(t *testing.T) {
	t.Parallel()

	base := logrus.New()
	base.SetFormatter(unsupportedFormatter{})

	l := New(base, "")

	err := l.ReportCaller()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown formatter type")
}
