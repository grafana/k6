package common

import (
	"testing"

	"github.com/chromedp/cdproto/page"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

func TestHandleFrameTreeNil(t *testing.T) {
	t.Parallel()

	fs := &FrameSession{logger: log.NewNullLogger()}
	require.NotPanics(t, func() {
		fs.handleFrameTree(nil, true)
		fs.handleFrameTree(&page.FrameTree{}, true)
		fs.handleFrameTree(&page.FrameTree{
			ChildFrames: []*page.FrameTree{nil},
		}, true)
	})
}
