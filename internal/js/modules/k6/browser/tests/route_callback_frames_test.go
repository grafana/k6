package tests

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"
)

func TestRouteCallbackFrames(t *testing.T) {
	t.Parallel()
	tb := newTestBrowser(t, withHTTPServer())
	p := tb.NewPage(nil)
	tb.withHandler("/route-frames", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, err := fmt.Fprint(w, "<body>route callback completed</body>")
		require.NoError(t, err)
	})
	observed := make(chan int, 1)
	require.NoError(t, p.Route(tb.url("/route-frames"), func(r *common.Route) error {
		observed <- len(p.Frames())
		return r.Continue(common.ContinueOptions{})
	}, func(pattern, rawURL string) (bool, error) { return pattern == rawURL, nil }))
	_, err := p.Goto(tb.url("/route-frames"), &common.FrameGotoOptions{
		WaitUntil: common.LifecycleEventDOMContentLoad, Timeout: 3 * time.Second,
	})
	require.NoError(t, err)
	require.Equal(t, 1, <-observed)
	require.NoError(t, p.Close())
}
