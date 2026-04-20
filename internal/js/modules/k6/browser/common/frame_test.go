package common

import (
	"context"
	"testing"
	"time"

	"github.com/chromedp/cdproto/target"

	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext/k6test"
	"go.k6.io/k6/internal/js/modules/k6/browser/log"

	"github.com/chromedp/cdproto/cdp"
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

func TestFrameWaitForExecutionContextReturnsWhenSessionCloses(t *testing.T) {
	t.Parallel()

	l := log.NewNullLogger()
	s := &sessionDoneTestStub{done: make(chan struct{})}
	fm := NewFrameManager(context.Background(), s, nil, NewTimeoutSettings(nil), l)
	frame := NewFrame(context.Background(), fm, nil, cdp.FrameID("42"), l)

	waitDone := make(chan struct{})
	go func() {
		frame.waitForExecutionContext(mainWorld)
		close(waitDone)
	}()

	close(s.done)

	select {
	case <-waitDone:
	case <-time.After(time.Second):
		require.FailNow(t, "waitForExecutionContext should return when session closes")
	}
}

func TestFrameWaitForExecutionContextReturnsWhenCallContextCloses(t *testing.T) {
	t.Parallel()

	l := log.NewNullLogger()
	fm := NewFrameManager(context.Background(), nil, nil, NewTimeoutSettings(nil), l)
	frame := NewFrame(context.Background(), fm, nil, cdp.FrameID("42"), l)

	callCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	waitDone := make(chan struct{})
	go func() {
		frame.waitForExecutionContextWithContext(callCtx, mainWorld)
		close(waitDone)
	}()

	select {
	case <-waitDone:
	case <-time.After(time.Second):
		require.FailNow(t, "waitForExecutionContextWithContext should return when call context closes")
	}
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

type sessionDoneTestStub struct {
	done chan struct{}
}

func (s *sessionDoneTestStub) Execute(context.Context, string, any, any) error {
	return nil
}

func (s *sessionDoneTestStub) emit(string, any) {}

func (s *sessionDoneTestStub) on(context.Context, []string, chan Event) {}

func (s *sessionDoneTestStub) onAll(context.Context, chan Event) {}

func (s *sessionDoneTestStub) ExecuteWithoutExpectationOnReply(context.Context, string, any, any) error {
	return nil
}

func (s *sessionDoneTestStub) ID() target.SessionID {
	return target.SessionID("session")
}

func (s *sessionDoneTestStub) TargetID() target.ID {
	return target.ID("target")
}

func (s *sessionDoneTestStub) Done() <-chan struct{} {
	return s.done
}

func (e *executionContextTestStub) eval( // this needs to be a pointer as otherwise it will copy the mutex inside of it
	apiCtx context.Context, opts evalOptions, js string, args ...any,
) (res any, err error) {
	return e.evalFn(apiCtx, opts, js, args...)
}

// toPtr is a helper function to convert a value to a pointer.
func toPtr[T any](v T) *T {
	return &v
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
			opts:      &GetByBaseOptions{Exact: toPtr(false)},
			want:      "internal:attr=[data-test='Foo Bar'i]",
		},
		{
			name:      "quoted_single_exact_true",
			attrName:  "data-test",
			attrValue: "'Foo Bar'",
			opts:      &GetByBaseOptions{Exact: toPtr(true)},
			want:      "internal:attr=[data-test='Foo Bar's]",
		},
		{
			name:      "quoted_double_exact_true",
			attrName:  "data-test",
			attrValue: "\"Foo Bar\"",
			opts:      &GetByBaseOptions{Exact: toPtr(true)},
			want:      "internal:attr=[data-test=\"Foo Bar\"s]",
		},
		{
			name:      "quoted_double_exact_false",
			attrName:  "data-test",
			attrValue: "\"Foo Bar\"",
			opts:      &GetByBaseOptions{Exact: toPtr(false)},
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
