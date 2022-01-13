package common

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
	"github.com/mailru/easyjson"
	"github.com/stretchr/testify/require"
)

func TestBrowserNewPageInContext(t *testing.T) {
	t.Parallel()

	type testCase struct {
		b   *Browser
		bc  *BrowserContext
		got struct {
			params easyjson.Marshaler
			method string
			page   *Page
			err    error
		}
	}
	newTestCase := func(id cdp.BrowserContextID) *testCase {
		ctx := context.Background()
		b := newBrowser(ctx, nil, nil, NewLaunchOptions(), NewNullLogger())
		// set a new browser context in the browser with `id`, so that newPageInContext can find it.
		b.contexts[id] = NewBrowserContext(ctx, b, id, nil, nil)

		tc := testCase{
			b:  b,
			bc: b.contexts[id],
		}

		return &tc
	}

	const (
		// default IDs to be used in tests.
		browserContextID cdp.BrowserContextID = "42"
		targetID         target.ID            = "84"

		// each test should finish in testTimeoutThreshold.
		testTimeoutThreshold = 100 * time.Millisecond
	)

	t.Run("happy_path", func(t *testing.T) {
		t.Parallel()

		// newPageInContext will look for this browser context.
		tc := newTestCase(browserContextID)

		// newPageInContext will return this page by searching it by its targetID in the wait event handler.
		tc.b.pages[targetID] = &Page{targetID: targetID}

		done := make(chan struct{})
		go func() {
			tc.b.conn = executorTestFunc(func(
				ctx context.Context, method string, params easyjson.Marshaler, res easyjson.Unmarshaler,
			) error {
				tc.got.params = params
				tc.got.method = method
				if method != target.CommandCreateTarget {
					// no need to continue the test if the command is not a create target action.
					return nil
				}

				// newPageInContext event handler will catch this target ID, and compare it to
				// the new page's target ID to detect whether the page is loaded.
				res.(*target.CreateTargetReturns).TargetID = targetID

				// for the event handler to work, there needs to be an event called
				// EventBrowserContextPage to be fired.
				//
				// this normally happens when the browser's onAttachedToTarget event is fired. here,
				// we imitate as if the browser created a target for the page.
				//
				//  but it's challenging to do that in this
				// unit test. besides, it's better to test lesser number of
				// units.
				tc.bc.emit(EventBrowserContextPage, &Page{targetID: targetID})

				return nil
			})
			tc.got.page, tc.got.err = tc.b.newPageInContext(browserContextID)

			done <- struct{}{}
		}()
		select {
		case <-done:
			require.NoError(t, tc.got.err)

			require.Equal(t, target.CommandCreateTarget, tc.got.method)
			params := tc.got.params.(*target.CreateTargetParams)
			require.Equal(t, "about:blank", params.URL)
			require.Equal(t, browserContextID, params.BrowserContextID)

			require.NotNil(t, tc.got.page)
			require.Equal(t, targetID, tc.got.page.targetID)
		case <-time.After(testTimeoutThreshold):
			// this may happen if the tc.b.conn above does not emit: EventBrowserContextPage.
			require.FailNow(t, "test timed out")
		}
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

	// should return the error returned from the executor.
	t.Run("error_in_create_target_action", func(t *testing.T) {
		t.Parallel()

		const wantErr = "anything"

		var (
			tc   = newTestCase(browserContextID)
			done = make(chan struct{})
		)
		go func() {
			tc.b.conn = executorTestFunc(
				func(context.Context, string, easyjson.Marshaler, easyjson.Unmarshaler) error {
					return errors.New(wantErr)
				})
			tc.got.page, tc.got.err = tc.b.newPageInContext(browserContextID)

			done <- struct{}{}
		}()
		select {
		case <-done:
			require.NotNil(t, tc.got.err)
			require.Contains(t, tc.got.err.Error(), wantErr)
			require.Nil(t, tc.got.page)
		case <-time.After(testTimeoutThreshold):
			require.FailNow(t, "test timed out")
		}
	})

	t.Run("timeout", func(t *testing.T) {
		t.Parallel()

		tc := newTestCase(browserContextID)

		// set a lower timeout for catching the timeout error.
		const timeout = 100 * time.Millisecond

		done := make(chan struct{})
		go func() {
			tc.b.conn = executorTestFunc(
				func(context.Context, string, easyjson.Marshaler, easyjson.Unmarshaler) error {
					// executor takes more time than the timeout.
					time.Sleep(2 * timeout)
					return nil
				})
			// set the timeout for the browser value.
			tc.b.launchOpts.Timeout = timeout
			// it should timeout in 100ms because the executor will sleep double of the timeout time.
			tc.got.page, tc.got.err = tc.b.newPageInContext(browserContextID)

			done <- struct{}{}
		}()
		select {
		case <-done:
			require.Error(t, tc.got.err)
			require.ErrorIs(t, tc.got.err, context.DeadlineExceeded)
			require.Nil(t, tc.got.page)
		case <-time.After(5 * timeout):
			require.FailNow(t, "test timed out: expected newPageInContext to time out instead")
		}
	})

	t.Run("context_done", func(t *testing.T) {
		t.Parallel()

		tc := newTestCase(browserContextID)

		done := make(chan struct{})
		go func() {
			tc.b.conn = executorTestFunc(
				func(context.Context, string, easyjson.Marshaler, easyjson.Unmarshaler) error {
					return nil
				})

			var cancel func()
			tc.b.ctx, cancel = context.WithCancel(tc.b.ctx)
			// let newPageInContext return a context cancelation error by canceling the context before
			// running the method.
			cancel()
			tc.got.page, tc.got.err = tc.b.newPageInContext(browserContextID)
			done <- struct{}{}

		}()
		select {
		case <-done:
			require.Error(t, tc.got.err)
			require.ErrorIs(t, tc.got.err, context.Canceled)
			require.Nil(t, tc.got.page)
		case <-time.After(testTimeoutThreshold):
			require.FailNow(t, "test timed out")
		}
	})
}

type executorTestFunc func(context.Context, string, easyjson.Marshaler, easyjson.Unmarshaler) error

func (f executorTestFunc) Execute(
	ctx context.Context, method string, params easyjson.Marshaler, res easyjson.Unmarshaler,
) error {
	return f(ctx, method, params, res)
}
