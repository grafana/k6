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
	l.ReportCaller()

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
