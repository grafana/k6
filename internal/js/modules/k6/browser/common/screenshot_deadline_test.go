package common

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

type screenshotDeadlineSession struct {
	session
	blocked              string
	backgroundColors     []*cdp.RGBA
	backgroundRestoreErr bool
}

const screenshotTimeout = 20 * time.Millisecond

func (s *screenshotDeadlineSession) Execute(ctx context.Context, method string, params, result any) error {
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
	case "Emulation.setDefaultBackgroundColorOverride":
		color := params.(*emulation.SetDefaultBackgroundColorOverrideParams).Color
		s.backgroundColors = append(s.backgroundColors, color)
		if s.backgroundRestoreErr && color == nil {
			return errors.New("background restore failed")
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

func TestScreenshotBehavior(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                 string
		blocked              string
		timeout, cancelAfter time.Duration
		wantErr              error
		omitBackground       bool
		backgroundRestoreErr bool
	}{
		{
			name: "deadline at layout metrics", blocked: "Page.getLayoutMetrics",
			timeout: screenshotTimeout, wantErr: context.DeadlineExceeded,
		},
		{
			name: "deadline at capture", blocked: "Page.captureScreenshot",
			timeout: screenshotTimeout, wantErr: context.DeadlineExceeded,
		},
		{
			name: "restores transparent background", blocked: "Page.captureScreenshot",
			timeout: screenshotTimeout, wantErr: context.DeadlineExceeded, omitBackground: true,
		},
		{
			name: "joins background restore error", blocked: "Page.captureScreenshot",
			timeout: screenshotTimeout, wantErr: context.DeadlineExceeded,
			omitBackground: true, backgroundRestoreErr: true,
		},
		{
			name: "parent cancellation with timeout", blocked: "Page.getLayoutMetrics",
			timeout: time.Second, cancelAfter: 20 * time.Millisecond, wantErr: context.Canceled,
		},
		{
			name: "parent cancellation with zero timeout", blocked: "Page.getLayoutMetrics",
			cancelAfter: 20 * time.Millisecond, wantErr: context.Canceled,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
				defer cancel()
				if tc.cancelAfter > 0 {
					go func() {
						time.Sleep(tc.cancelAfter)
						cancel()
					}()
				}
				sess := &screenshotDeadlineSession{
					session: &Session{}, blocked: tc.blocked,
					backgroundRestoreErr: tc.backgroundRestoreErr,
				}
				p := testScreenshotPage(ctx, sess, tc.timeout)
				opts := NewPageScreenshotOptions()
				opts.OmitBackground = tc.omitBackground

				start := time.Now()
				_, err := p.Screenshot(opts, nil)
				require.ErrorIs(t, err, tc.wantErr)
				require.Equal(t, screenshotTimeout, time.Since(start))
				if errors.Is(tc.wantErr, context.DeadlineExceeded) {
					require.ErrorContains(t, err, "timed out after 20ms")
				}
				if tc.omitBackground {
					require.Equal(t, []*cdp.RGBA{{A: 0}, nil}, sess.backgroundColors)
				}
				if tc.backgroundRestoreErr {
					require.ErrorContains(t, err, "resetting screenshot background color: background restore failed")
				}
				if tc.cancelAfter > 0 {
					require.ErrorIs(t, ctx.Err(), context.Canceled)
				} else {
					require.NoError(t, ctx.Err())
				}
			})
		})
	}
}

func TestScreenshotRestoreUsesBoundedCleanup(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()
		sess := &screenshotDeadlineSession{session: &Session{}, blocked: "Emulation.setDeviceMetricsOverride"}
		logger := log.NewNullLogger()
		temporarySize := NewEmulatedSize(Viewport{Width: 300, Height: 300}, Screen{Width: 300, Height: 300})
		p := &Page{
			ctx: ctx, session: sess, logger: logger, emulatedSize: temporarySize,
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
		require.Same(t, temporarySize, p.emulatedSize, "failed restoration must not update the cached viewport")
	})
}
