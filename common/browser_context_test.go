package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/xk6-browser/common/js"
	"github.com/grafana/xk6-browser/k6ext"
	"github.com/grafana/xk6-browser/k6ext/k6test"
	"github.com/grafana/xk6-browser/log"
)

func TestNewBrowserContext(t *testing.T) {
	t.Parallel()

	t.Run("add_web_vital_js_scripts_to_context", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		logger := log.NewNullLogger()
		b := newBrowser(ctx, cancel, nil, NewLocalBrowserOptions(), logger)

		vu := k6test.NewVU(t)
		ctx = k6ext.WithVU(ctx, vu)

		bc, err := NewBrowserContext(ctx, b, "some-id", nil, nil)
		require.NoError(t, err)

		webVitalIIFEScriptFound := false
		webVitalInitScriptFound := false
		k6ObjScriptFound := false
		for _, script := range bc.evaluateOnNewDocumentSources {
			switch script {
			case js.WebVitalIIFEScript:
				webVitalIIFEScriptFound = true
			case js.WebVitalInitScript:
				webVitalInitScriptFound = true
			case js.K6ObjectScript:
				k6ObjScriptFound = true
			default:
				assert.Fail(t, "script is neither WebVitalIIFEScript, WebVitalInitScript, nor k6ObjScript")
			}
		}

		assert.True(t, webVitalIIFEScriptFound, "WebVitalIIFEScript was not initialized in the context")
		assert.True(t, webVitalInitScriptFound, "WebVitalInitScript was not initialized in the context")
		assert.True(t, k6ObjScriptFound, "k6ObjScript was not initialized in the context")
	})
}
