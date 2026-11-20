package common

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext/k6test"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/runtime"
	"github.com/stretchr/testify/require"
)

// Test calling Frame.document does not panic with a nil document.
// See: Issue #53 for details.
func TestFrameNilDocument(t *testing.T) {
	t.Parallel()

	vu := k6test.NewVU(t)
	log := log.NewNullLogger()

	fm := NewFrameManager(vu.Context(), nil, nil, nil, log)
	frame := NewFrame(vu.Context(), fm, nil, cdp.FrameID("42"), log)

	// frame should not panic with a nil document
	stub := &executionContextTestStub{
		evalFn: func(apiCtx context.Context, opts evalOptions, js string, args ...any) (res any, err error) {
			// return nil to test for panic
			return nil, nil //nolint:nilnil
		},
	}

	// document() waits for the main execution context
	ok := make(chan struct{}, 1)
	go func() {
		frame.setContext(mainWorld, stub)
		ok <- struct{}{}
	}()
	select {
	case <-ok:
	case <-time.After(time.Second):
		require.FailNow(t, "cannot set the main execution context, frame.setContext timed out")
	}

	require.NotPanics(t, func() {
		_, err := frame.document()
		require.Error(t, err)
	})

	// frame gets the document from the evaluate call
	want := &ElementHandle{}
	stub.evalFn = func(
		apiCtx context.Context, opts evalOptions, js string, args ...any,
	) (res any, err error) {
		return want, nil
	}
	got, err := frame.document()
	require.NoError(t, err)
	require.Equal(t, want, got)

	// frame sets documentHandle in the document method
	got = frame.documentHandle
	require.Equal(t, want, got)
}

// See: Issue #177 for details.
func TestFrameManagerFrameAbortedNavigationShouldEmitANonNilPendingDocument(t *testing.T) {
	t.Parallel()

	ctx, log := context.Background(), log.NewNullLogger()

	// add the frame to frame manager
	fm := NewFrameManager(ctx, nil, nil, NewTimeoutSettings(nil), log)
	frame := NewFrame(ctx, fm, nil, cdp.FrameID("42"), log)
	fm.frames[frame.id] = frame

	// listen for frame navigation events
	recv := make(chan Event)
	frame.on(ctx, []string{EventFrameNavigation}, recv)

	// emit the navigation event
	frame.pendingDocument = &DocumentInfo{
		documentID: "42",
	}
	fm.frameAbortedNavigation(frame.id, "any error", frame.pendingDocument.documentID)

	// receive the emitted event and verify that emitted document
	// is not nil.
	e := <-recv
	require.IsType(t, &NavigationEvent{}, e.data, "event should be a navigation event")
	ne := e.data.(*NavigationEvent)
	require.NotNil(t, ne, "event should not be nil")
	require.NotNil(t, ne.newDocument, "emitted document should not be nil")

	// since the navigation is aborted, the aborting frame should have
	// a nil pending document.
	require.Nil(t, frame.pendingDocument)
}

type executionContextTestStub struct {
	ExecutionContext
	evalFn func(
		apiCtx context.Context, opts evalOptions, js string, args ...any,
	) (res any, err error)
}

func (e *executionContextTestStub) eval( // this needs to be a pointer as otherwise it will copy the mutex inside of it
	apiCtx context.Context, opts evalOptions, js string, args ...any,
) (res any, err error) {
	return e.evalFn(apiCtx, opts, js, args...)
}

func TestBuildAttributeSelector(t *testing.T) {
	t.Parallel()

	f := &Frame{}

	tests := []struct {
		name      string
		attrName  string
		attrValue string
		opts      *GetByBaseOptions
		want      string
	}{
		{
			name:      "empty",
			attrName:  "",
			attrValue: "",
			opts:      nil,
			want:      "internal:attr=[=]",
		},
		{
			name:      "unquoted_no_opts",
			attrName:  "data-test",
			attrValue: "foo",
			opts:      nil,
			want:      "internal:attr=[data-test=foo]",
		},
		{
			name:      "quoted_single_nil_opts",
			attrName:  "data-test",
			attrValue: "'Foo Bar'",
			opts:      nil,
			want:      "internal:attr=[data-test='Foo Bar'i]",
		},
		{
			name:      "quoted_single_exact_false",
			attrName:  "data-test",
			attrValue: "'Foo Bar'",
			opts:      &GetByBaseOptions{Exact: new(false)},
			want:      "internal:attr=[data-test='Foo Bar'i]",
		},
		{
			name:      "quoted_single_exact_true",
			attrName:  "data-test",
			attrValue: "'Foo Bar'",
			opts:      &GetByBaseOptions{Exact: new(true)},
			want:      "internal:attr=[data-test='Foo Bar's]",
		},
		{
			name:      "quoted_double_exact_true",
			attrName:  "data-test",
			attrValue: "\"Foo Bar\"",
			opts:      &GetByBaseOptions{Exact: new(true)},
			want:      "internal:attr=[data-test=\"Foo Bar\"s]",
		},
		{
			name:      "quoted_double_exact_false",
			attrName:  "data-test",
			attrValue: "\"Foo Bar\"",
			opts:      &GetByBaseOptions{Exact: new(false)},
			want:      "internal:attr=[data-test=\"Foo Bar\"i]",
		},
		{
			name:      "quoted_single_exact_nil",
			attrName:  "data-test",
			attrValue: "'Foo Bar'",
			opts:      &GetByBaseOptions{Exact: nil},
			want:      "internal:attr=[data-test='Foo Bar'i]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := f.buildAttributeSelector(tc.attrName, tc.attrValue, tc.opts)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestIsQuotedText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "empty", in: "", want: false},
		{name: "unquoted", in: "foo", want: false},
		{name: "single_quoted", in: "'foo'", want: true},
		{name: "double_quoted", in: "\"foo\"", want: true},
		{name: "mismatched_quotes_1", in: "'foo\"", want: false},
		{name: "mismatched_quotes_2", in: "\"foo'", want: false},
		{name: "just_single_quote", in: "'", want: false},
		{name: "just_double_quote", in: "\"", want: false},
		{name: "two_single_quotes", in: "''", want: true},
		{name: "two_double_quotes", in: "\"\"", want: true},
		{name: "leading_space_then_quoted", in: " 'foo'", want: true},
		{name: "trailing_space_after_quoted", in: "'foo' ", want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := isQuotedText(tc.in)
			if got != tc.want {
				t.Fatalf("isQuotedText(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestFrameActionsReleaseHandle(t *testing.T) {
	t.Parallel()
	actionErr := errors.New("action failed")
	lookupErr := errors.New("selector evaluation failed")
	for _, pointer := range []bool{false, true} {
		name := "action"
		if pointer {
			name = "pointer_action"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for _, tc := range []struct {
				name                 string
				lookupErr, actionErr error
			}{
				{name: "success"},
				{name: "action_error", actionErr: actionErr},
				{name: "lookup_error", lookupErr: lookupErr},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					s := &handleReleaseSession{lookupErr: tc.lookupErr}
					frame := newHandleReleaseFrame(t, s)
					action := func(_ context.Context, handle *ElementHandle) (any, error) {
						// Check lifetime while the callback is using the acquired handle.
						s.events = append(s.events, "action")
						if handle.disposed || len(s.released) != 0 {
							return nil, errors.New("handle released before action")
						}
						return "result", tc.actionErr
					}
					run := frame.newAction("#target", DOMElementStateAttached, true, action,
						nil, true, false, true, time.Second)
					if pointer {
						opts := &ElementHandleBasePointerOptions{}
						opts.Force = true
						opts.NoWaitAfter = true
						opts.Timeout = time.Second
						run = frame.newPointerAction("#target", DOMElementStateAttached, true,
							func(ctx context.Context, handle *ElementHandle, _ *Position) (any, error) {
								return action(ctx, handle)
							}, opts)
					}
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					results, errs := make(chan any, 1), make(chan error, 1)
					done := make(chan struct{})
					go func() {
						defer close(done)
						run(ctx, results, errs)
					}()
					// Result delivery can precede disposal. Wait for the whole wrapper to exit.
					select {
					case <-done:
					case <-ctx.Done():
						t.Fatal("action wrapper did not finish")
					}
					expectedErr := tc.actionErr
					if tc.lookupErr != nil {
						expectedErr = tc.lookupErr
					}
					if expectedErr != nil {
						select {
						case err := <-errs:
							require.ErrorIs(t, err, expectedErr)
						default:
							t.Fatal("expected action error")
						}
						require.Empty(t, results)
					} else {
						select {
						case result := <-results:
							require.Equal(t, "result", result)
						default:
							t.Fatal("expected action result")
						}
						require.Empty(t, errs)
					}
					if tc.lookupErr != nil {
						require.Empty(t, s.events)
						require.Empty(t, s.released)
					} else {
						require.Equal(t, []string{"action", "release"}, s.events)
						require.Equal(t, []runtime.RemoteObjectID{"temporary-element"}, s.released)
					}
				})
			}
		})
	}
}

func TestFrameActionReleasesHandleAfterRetry(t *testing.T) {
	t.Parallel()
	s := &handleReleaseSession{}
	frame := newHandleReleaseFrame(t, s)
	var firstHandle *ElementHandle
	attempts := 0
	action := func(_ context.Context, handle *ElementHandle) (any, error) {
		attempts++
		s.events = append(s.events, "action")
		if handle.disposed || len(s.released) != 0 {
			return nil, errors.New("handle released before retry completed")
		}
		if attempts == 1 {
			firstHandle = handle
			// This sentinel triggers the real retry loop without relying on DOM timing.
			return nil, ErrElementNotAttachedToDOM
		}
		if handle != firstHandle {
			return nil, errors.New("retry did not reuse the acquired handle")
		}
		return "retried successfully", nil
	}
	// Force must be false: forcing an action disables retries.
	run := frame.newAction("#target", DOMElementStateAttached, true, action,
		[]string{"attached"}, false, true, true, time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	results, errs := make(chan any, 1), make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		run(ctx, results, errs)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("retrying action did not finish")
	}
	require.Empty(t, errs)
	select {
	case result := <-results:
		require.Equal(t, "retried successfully", result)
	default:
		t.Fatal("expected successful retry result")
	}
	require.Equal(t, 2, attempts)
	require.Equal(t, []string{"action", "action", "release"}, s.events)
	require.Equal(t, []runtime.RemoteObjectID{"temporary-element"}, s.released)
}
