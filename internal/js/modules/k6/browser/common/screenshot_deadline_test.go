package common

import (
	"context"
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

type screenshotDeadlineSession struct {
	session
	blocked string
}

func (s *screenshotDeadlineSession) Execute(ctx context.Context, method string, _, result any) error {
	if method == s.blocked {
		<-ctx.Done()
		return ctx.Err()
	}
	if method != "Page.getLayoutMetrics" {
		panic(method)
	}
	// Time spent preparing the screenshot must consume the same deadline as capture.
	time.Sleep(5 * time.Millisecond)
	result.(*page.GetLayoutMetricsReturns).CSSVisualViewport = &page.VisualViewport{Scale: 1}
	return nil
}

func TestScreenshotDeadline(t *testing.T) {
	t.Parallel()
	for _, method := range []string{"Page.getLayoutMetrics", "Page.captureScreenshot"} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
				defer cancel()
				settings := NewTimeoutSettings(nil)
				settings.setDefaultTimeout(20 * time.Millisecond)
				p := &Page{
					ctx: ctx, session: &screenshotDeadlineSession{blocked: method},
					timeoutSettings: settings, logger: log.NewNullLogger(),
					emulatedSize: NewEmulatedSize(Viewport{Width: 100, Height: 100}, Screen{Width: 100, Height: 100}),
				}
				start := time.Now()
				_, err := p.Screenshot(NewPageScreenshotOptions(), nil)
				require.ErrorIs(t, err, context.DeadlineExceeded)
				require.Equal(t, 20*time.Millisecond, time.Since(start))
				require.NoError(t, ctx.Err(), "screenshot deadline must not cancel the page")
			})
		})
	}
}

func TestScreenshotRestoreDeadline(t *testing.T) {
	t.Parallel()
	for _, viewport := range []*Size{nil, {Width: 100, Height: 100}} {
		t.Run(fmt.Sprint(viewport), func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
				defer cancel()
				logger := log.NewNullLogger()
				sess := &screenshotDeadlineSession{session: &Session{}, blocked: "Emulation.setDeviceMetricsOverride"}
				p := &Page{ctx: ctx, session: sess, logger: logger, browserCtx: &BrowserContext{opts: DefaultBrowserContextOptions()}}
				p.mainFrameSession = &FrameSession{ctx: ctx, session: sess, page: p, logger: logger}
				s, stop := newScreenshotter(ctx, 20*time.Millisecond, nil, logger)
				defer stop()
				start := time.Now()
				require.ErrorIs(t, s.restoreViewport(p, viewport), context.DeadlineExceeded)
				require.Equal(t, 20*time.Millisecond, time.Since(start))
				require.NoError(t, ctx.Err())
			})
		})
	}
}

func TestScreenshotCancellation(t *testing.T) {
	t.Parallel()
	for _, timeout := range []time.Duration{0, time.Second} {
		t.Run(timeout.String(), func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				s, stop := newScreenshotter(ctx, timeout, nil, log.NewNullLogger())
				defer stop()
				if timeout == 0 {
					_, ok := s.ctx.Deadline()
					require.False(t, ok)
				}
				go func() { time.Sleep(10 * time.Millisecond); cancel() }()
				start := time.Now()
				_, err := s.screenshot(&screenshotDeadlineSession{blocked: "Page.getLayoutMetrics"}, nil, &Rect{Width: 100, Height: 100}, ImageFormatPNG, false, 0, "")
				require.ErrorIs(t, err, context.Canceled)
				require.Equal(t, 10*time.Millisecond, time.Since(start))
			})
		})
	}
}
