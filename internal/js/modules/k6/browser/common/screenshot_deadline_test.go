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
	blocked                string
	backgroundOverrides    int
	backgroundRestoreError bool
}

func (s *screenshotDeadlineSession) Execute(ctx context.Context, method string, _, result any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if method == s.blocked {
		<-ctx.Done()
		return ctx.Err()
	}
	switch method {
	case "Page.getLayoutMetrics":
		result.(*page.GetLayoutMetricsReturns).CSSVisualViewport = &page.VisualViewport{Scale: 1}
	case "Page.captureScreenshot":
		result.(*page.CaptureScreenshotReturns).Data = ""
	case "Emulation.setDefaultBackgroundColorOverride":
		s.backgroundOverrides++
		if s.backgroundRestoreError && s.backgroundOverrides == 2 {
			return fmt.Errorf("background restore failed")
		}
	case "Emulation.setDeviceMetricsOverride":
	default:
		return fmt.Errorf("unexpected CDP method %q", method)
	}
	return nil
}

func testScreenshotPage(ctx context.Context, sess session, timeout time.Duration) *Page {
	settings := NewTimeoutSettings(nil)
	settings.setDefaultTimeout(timeout)
	return &Page{
		ctx: ctx, session: sess, timeoutSettings: settings, logger: log.NewNullLogger(),
		emulatedSize: NewEmulatedSize(Viewport{Width: 100, Height: 100}, Screen{Width: 100, Height: 100}),
	}
}

func TestScreenshotDeadline(t *testing.T) {
	t.Parallel()
	for _, method := range []string{"Page.getLayoutMetrics", "Page.captureScreenshot"} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
				defer cancel()
				sess := &screenshotDeadlineSession{session: &Session{}, blocked: method}
				p := testScreenshotPage(ctx, sess, 20*time.Millisecond)

				start := time.Now()
				_, err := p.Screenshot(NewPageScreenshotOptions(), nil)
				t.Logf("elapsed=%s parentErr=%v", time.Since(start), ctx.Err())
				require.ErrorIs(t, err, context.DeadlineExceeded)
				require.ErrorContains(t, err, "timed out after 20ms")
				require.Equal(t, 20*time.Millisecond, time.Since(start))
				require.NoError(t, ctx.Err())
			})
		})
	}
}

func TestScreenshotRestoresBackgroundAfterCaptureError(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		sess := &screenshotDeadlineSession{session: &Session{}, blocked: "Page.captureScreenshot"}
		p := testScreenshotPage(ctx, sess, 20*time.Millisecond)
		opts := NewPageScreenshotOptions()
		opts.OmitBackground = true

		_, err := p.Screenshot(opts, nil)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Equal(t, 2, sess.backgroundOverrides)
		require.NoError(t, ctx.Err())
	})
}

func TestScreenshotRestoreUsesBoundedCleanup(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()
		sess := &screenshotDeadlineSession{session: &Session{}, blocked: "Emulation.setDeviceMetricsOverride"}
		logger := log.NewNullLogger()
		p := &Page{
			ctx: ctx, session: sess, logger: logger,
			browserCtx: &BrowserContext{opts: DefaultBrowserContextOptions()},
		}
		p.mainFrameSession = &FrameSession{ctx: ctx, session: sess, page: p, logger: logger}
		s, stop := newScreenshotter(ctx, 20*time.Millisecond, nil, logger)
		stop()

		start := time.Now()
		err := s.restoreViewport(p, NewEmulatedSize(Viewport{Width: 100, Height: 100}, Screen{Width: 200, Height: 200}))
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Equal(t, screenshotCleanupTimeout, time.Since(start))
		require.NoError(t, ctx.Err())
	})
}

func TestScreenshotPreservesCaptureAndCleanupErrors(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		sess := &screenshotDeadlineSession{
			session: &Session{}, blocked: "Page.captureScreenshot", backgroundRestoreError: true,
		}
		p := testScreenshotPage(ctx, sess, 20*time.Millisecond)
		opts := NewPageScreenshotOptions()
		opts.OmitBackground = true

		_, err := p.Screenshot(opts, nil)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.ErrorContains(t, err, "resetting screenshot background color: background restore failed")
	})
}

func TestScreenshotPreservesParentCancellation(t *testing.T) {
	t.Parallel()
	for _, timeout := range []time.Duration{0, time.Second} {
		t.Run(timeout.String(), func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				go func() {
					time.Sleep(20 * time.Millisecond)
					cancel()
				}()
				s, stop := newScreenshotter(ctx, timeout, nil, log.NewNullLogger())
				defer stop()
				if timeout == 0 {
					_, hasDeadline := s.ctx.Deadline()
					require.False(t, hasDeadline)
				}

				start := time.Now()
				_, err := s.screenshot(
					&screenshotDeadlineSession{blocked: "Page.getLayoutMetrics"},
					nil, &Rect{Width: 100, Height: 100}, ImageFormatPNG, false, 0, "",
				)
				require.ErrorIs(t, err, context.Canceled)
				require.Equal(t, 20*time.Millisecond, time.Since(start))
				require.ErrorIs(t, ctx.Err(), context.Canceled)
			})
		})
	}
}

func TestSetEmulatedSizeKeepsCachedSizeOnError(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		logger := log.NewNullLogger()
		sess := &screenshotDeadlineSession{session: &Session{}, blocked: "Emulation.setDeviceMetricsOverride"}
		original := NewEmulatedSize(Viewport{Width: 100, Height: 100}, Screen{Width: 200, Height: 200})
		p := &Page{
			ctx: ctx, session: sess, logger: logger, emulatedSize: original,
			browserCtx: &BrowserContext{opts: DefaultBrowserContextOptions()},
		}
		p.mainFrameSession = &FrameSession{ctx: ctx, session: sess, page: p, logger: logger}

		err := p.setEmulatedSize(ctx, NewEmulatedSize(
			Viewport{Width: 300, Height: 300}, Screen{Width: 300, Height: 300},
		))
		require.ErrorIs(t, err, context.Canceled)
		require.Same(t, original, p.emulatedSize)
	})
}
