package common

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext/k6test"
	"go.k6.io/k6/internal/js/modules/k6/browser/log"
)

func TestBrowserNewPageInContext(t *testing.T) {
	t.Parallel()

	const (
		// default IDs to be used in tests.
		browserContextID cdp.BrowserContextID = "42"
		targetID         target.ID            = "84"
	)

	type testCase struct {
		b  *Browser
		bc *BrowserContext
	}

	newTestCase := func(id cdp.BrowserContextID) *testCase {
		ctx, cancel := context.WithCancel(context.Background())
		b := newBrowser(context.Background(), ctx, cancel, nil, NewLocalBrowserOptions(), log.NewNullLogger())

		// set a new browser context in the browser with `id`, so that newPageInContext can find it.
		var err error
		b.context, err = NewBrowserContext(k6ext.WithVU(ctx, k6test.NewVU(t)), b, id, nil, nil)
		b.defaultContext = b.context // always happens when a new browser is connected
		require.NoError(t, err)

		tc := &testCase{b: b, bc: b.context}

		// newPageInContext will return this page by searching it by its targetID in the wait event handler.
		tc.b.pages[targetID] = &Page{targetID: targetID}
		tc.b.conn = fakeConn{
			execute: func(ctx context.Context, method string, params, res any) error {
				require.Equal(t, target.CommandCreateTarget, method)
				require.IsType(t, params, &target.CreateTargetParams{})
				tp, _ := params.(*target.CreateTargetParams)
				require.Equal(t, BlankPage, tp.URL)
				require.Equal(t, browserContextID, tp.BrowserContextID)

				// newPageInContext event handler will catch this target ID, and compare it to
				// the new page's target ID to detect whether the page
				// is loaded.
				require.IsType(t, res, &target.CreateTargetReturns{})
				v, _ := res.(*target.CreateTargetReturns)
				v.TargetID = targetID

				// for the event handler to work, there needs to be an event called
				// EventBrowserContextPage to be fired. this normally happens when the browser's
				// onAttachedToTarget event is fired. here, we imitate as if the browser created a target for
				// the page.
				tc.bc.emit(EventBrowserContextPage, &Page{targetID: targetID})

				return nil
			},
		}
		return tc
	}

	t.Run("happy_path", func(t *testing.T) {
		t.Parallel()

		// newPageInContext will look for this browser context.
		tc := newTestCase(browserContextID)

		page, err := tc.b.newPageInContext(browserContextID)
		require.NoError(t, err)
		require.NotNil(t, page)
		require.Equal(t, targetID, page.targetID)
	})

	// should return an error if it cannot find a browser context.
	t.Run("missing_browser_context", func(t *testing.T) {
		t.Parallel()

		const missingBrowserContextID = "911"

		// set an existing browser context,
		_, err := newTestCase(browserContextID).
			// but look for a different one.
			b.newPageInContext(missingBrowserContextID)
		require.Error(t, err)
		require.Contains(t, err.Error(), missingBrowserContextID,
			"should have returned the missing browser context ID in the error message")
	})

	t.Run("uses_default_browser_context", func(t *testing.T) {
		t.Parallel()

		tc := newTestCase(browserContextID)
		tc.b.context = nil // should use default context if there is no current context

		require.NotPanics(t, func() {
			_, err := tc.b.newPageInContext(browserContextID)
			require.NoError(t, err)
		})
	})

	// should return the error returned from the executor.
	t.Run("error_in_create_target_action", func(t *testing.T) {
		t.Parallel()

		const wantErr = "anything"

		tc := newTestCase(browserContextID)
		tc.b.conn = fakeConn{
			execute: func(context.Context, string, any, any) error {
				return errors.New(wantErr)
			},
		}
		page, err := tc.b.newPageInContext(browserContextID)

		require.NotNil(t, err)
		require.Contains(t, err.Error(), wantErr)
		require.Nil(t, page)
	})

	t.Run("timeout", func(t *testing.T) {
		t.Parallel()

		tc := newTestCase(browserContextID)

		// set a lower timeout for catching the timeout error.
		const timeout = 100 * time.Millisecond
		// set the timeout for the browser value.
		tc.b.browserOpts.Timeout = timeout
		tc.b.conn = fakeConn{
			execute: func(context.Context, string, any, any) error {
				// executor takes more time than the timeout.
				time.Sleep(2 * timeout)
				return nil
			},
		}

		var (
			page *Page
			err  error

			done = make(chan struct{})
		)
		go func() {
			// it should timeout in 100ms because the executor will sleep double of the timeout time.
			page, err = tc.b.newPageInContext(browserContextID)
			done <- struct{}{}
		}()
		select {
		case <-done:
			require.Error(t, err)
			require.ErrorIs(t, err, context.DeadlineExceeded)
			require.Nil(t, page)
		case <-time.After(5 * timeout):
			require.FailNow(t, "test timed out: expected newPageInContext to time out instead")
		}
	})

	t.Run("context_done", func(t *testing.T) {
		t.Parallel()

		tc := newTestCase(browserContextID)

		tc.b.conn = fakeConn{
			execute: func(context.Context, string, any, any) error {
				return nil
			},
		}

		var cancel func()
		tc.b.vuCtx, cancel = context.WithCancel(tc.b.vuCtx)
		// let newPageInContext return a context cancelation error by canceling the context before
		// running the method.
		cancel()
		page, err := tc.b.newPageInContext(browserContextID)
		require.Error(t, err)
		require.ErrorIs(t, err, context.Canceled)
		require.Nil(t, page)
	})
}

type fakeConn struct {
	connection
	execute func(context.Context, string, any, any) error
}

func (c fakeConn) Execute(
	ctx context.Context, method string, params, res any,
) error {
	return c.execute(ctx, method, params, res)
}
