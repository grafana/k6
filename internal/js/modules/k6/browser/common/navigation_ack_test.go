package common

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

type navigationACKSession struct {
	session
	delay       time.Duration
	nilOnCancel bool
}

func (s *navigationACKSession) Execute(ctx context.Context, method string, _, result any) error {
	if method != "Page.navigate" {
		panic(method)
	}
	select {
	case <-ctx.Done():
		if s.nilOnCancel {
			return nil
		}
		return ctx.Err()
	case <-time.After(s.delay):
	}
	result.(*page.NavigateReturns).FrameID = "frame"
	return nil
}

func TestNavigationACKDeadline(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                     string
		delay, cancelAfter, want time.Duration
		wantError                bool
		nilOnCancel              bool
	}{
		{"normal same document", 5 * time.Millisecond, 0, 5 * time.Millisecond, false, false},
		{"delayed ACK", 60 * time.Millisecond, 0, 20 * time.Millisecond, true, false},
		{"page cancellation", 60 * time.Millisecond, 10 * time.Millisecond, 10 * time.Millisecond, true, false},
		{"cancelled without result", 60 * time.Millisecond, 10 * time.Millisecond, 10 * time.Millisecond, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				if tc.cancelAfter > 0 {
					go func() { time.Sleep(tc.cancelAfter); cancel() }()
				}
				logger := log.NewNullLogger()
				s := &navigationACKSession{session: &Session{id: "session"}, delay: tc.delay, nilOnCancel: tc.nilOnCancel}
				fs := &FrameSession{ctx: ctx, session: s, logger: logger}
				p := &Page{logger: logger, mainFrameSession: fs, frameSessions: map[cdp.FrameID]*FrameSession{"frame": fs}}
				m := NewFrameManager(ctx, s, p, NewTimeoutSettings(nil), logger)
				f := NewFrame(ctx, m, nil, "frame", logger)
				start := time.Now()
				_, err := m.NavigateFrame(f, "https://owned.invalid/", &FrameGotoOptions{Timeout: 20 * time.Millisecond})
				require.Equal(t, tc.want, time.Since(start))
				if tc.wantError {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			})
		})
	}
}
