package tests

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"
)

func TestBrowserContextPagesOrder(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)
	first := tb.NewPage(nil)
	bctx := first.Context()
	second, err := bctx.NewPage()
	require.NoError(t, err)
	third, err := bctx.NewPage()
	require.NoError(t, err)

	checkOrder := func(want ...*common.Page) {
		t.Helper()
		for range 32 {
			pages := bctx.Pages()
			require.Len(t, pages, len(want))
			for i, page := range want {
				require.True(t, page == pages[i], "page at index %d", i)
			}
		}
	}

	checkOrder(first, second, third)
	require.NoError(t, second.Close())
	require.Eventually(t, func() bool { return len(bctx.Pages()) == 2 }, 5*time.Second, 10*time.Millisecond)
	checkOrder(first, third)
	fourth, err := bctx.NewPage()
	require.NoError(t, err)
	checkOrder(first, third, fourth)
}
