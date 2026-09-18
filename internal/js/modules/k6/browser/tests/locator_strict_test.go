package tests

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"
)

func TestLocatorIgnoresStrictFalse(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	_, err := p.Goto(tb.staticURL("locators.html"), &common.FrameGotoOptions{
		Timeout: common.DefaultTimeout,
	})
	require.NoError(t, err)

	opts := common.NewFrameClickOptions(100 * time.Millisecond)
	opts.Strict = false
	err = p.Locator("a", nil).Click(opts)
	require.Error(t, err)
}
