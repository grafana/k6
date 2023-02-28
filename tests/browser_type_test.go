package tests

import (
	"testing"

	"github.com/grafana/xk6-browser/chromium"
	"github.com/grafana/xk6-browser/k6ext/k6test"
)

func TestBrowserTypeConnect(t *testing.T) {
	// Start a test browser so we can get its WS URL
	// and use it to connect through BrowserType.Connect.
	tb := newTestBrowser(t)
	vu := k6test.NewVU(t)
	bt := chromium.NewBrowserType(vu)
	vu.MoveToVUContext()

	b := bt.Connect(tb.wsURL, nil)
	b.NewPage(nil)
}
