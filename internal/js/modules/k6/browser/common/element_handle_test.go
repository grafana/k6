package common

import (
	"context"
	"errors"
	"testing"

	"go.k6.io/k6/internal/js/modules/k6/browser/common/js"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorFromDOMError(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		in       string
		sentinel bool // if it returns the same error value
		want     error
	}{
		{in: "timed out", want: ErrTimedOut, sentinel: true},
		{in: "error:notconnected", want: errors.New("element is not attached to the DOM")},
		{in: "error:expectednode:anything", want: errors.New("expected node but got anything")},
		{in: "nonexistent error", want: errors.New("nonexistent error")},
	} {
		got := errorFromDOMError(tc.in)
		if tc.sentinel && !errors.Is(got, tc.want) {
			assert.Failf(t, "not sentinel", "error value of %q should be sentinel", tc.in)
		} else {
			require.Error(t, got)
			assert.EqualError(t, tc.want, got.Error())
		}
	}
}

func TestQueryAll(t *testing.T) {
	t.Parallel()

	var (
		nilHandle    = func() *ElementHandle { return nil }
		nonNilHandle = func() *ElementHandle { return &ElementHandle{} }
	)

	for name, tt := range map[string]struct {
		selector                      string
		returnHandle                  func() any
		returnErr                     error
		wantErr                       bool
		wantHandles, wantDisposeCalls int
	}{
		"invalid_selector": {
			selector:     "*=*>>*=*",
			returnHandle: func() any { return nil },
			returnErr:    errors.New("any"),
			wantErr:      true,
		},
		"cannot_evaluate": {
			selector:     "*",
			returnHandle: func() any { return nil },
			returnErr:    errors.New("any"),
			wantErr:      true,
		},
		"nil_handles_no_err": {
			selector:     "*",
			returnHandle: func() any { return nil },
			returnErr:    nil,
			wantErr:      false,
		},
		"invalid_js_handle": {
			selector:     "*",
			returnHandle: func() any { return "an invalid handle" },
			wantErr:      true,
		},
		"disposes_main_handle": {
			selector:         "*",
			returnHandle:     func() any { return &jsHandleStub{} },
			wantDisposeCalls: 1,
		},
		// disposes the main handle and all its nil children
		"disposes_handles": {
			selector: "*",
			returnHandle: func() any {
				handles := &jsHandleStub{
					asElementFn: nilHandle,
				}
				handles.getPropertiesFn = func() (map[string]JSHandleAPI, error) {
					return map[string]JSHandleAPI{
						"1": handles,
						"2": handles,
					}, nil
				}

				return handles
			},
			wantDisposeCalls: 3,
		},
		// only returns non-nil handles
		"returns_elems": {
			selector: "*",
			returnHandle: func() any {
				childHandles := map[string]JSHandleAPI{
					"1": &jsHandleStub{asElementFn: nonNilHandle},
					"2": &jsHandleStub{asElementFn: nonNilHandle},
					"3": &jsHandleStub{asElementFn: nilHandle},
				}
				return &jsHandleStub{
					getPropertiesFn: func() (map[string]JSHandleAPI, error) {
						return childHandles, nil
					},
				}
			},
			wantHandles: 2,
		},
		"returns_err": {
			selector: "*",
			returnHandle: func() any {
				return &jsHandleStub{
					getPropertiesFn: func() (map[string]JSHandleAPI, error) {
						return nil, errors.New("any error")
					},
				}
			},
			wantHandles: 0,
			wantErr:     true,
		},
	} {
		name, tt := name, tt

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var (
				returnHandle = tt.returnHandle()
				evalFunc     = func(_ context.Context, _ evalOptions, _ string, _ ...any) (any, error) {
					return returnHandle, tt.returnErr
				}
			)

			handles, err := (&ElementHandle{}).queryAll(tt.selector, evalFunc)

			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, handles)
			} else {
				require.NoError(t, err)
			}
			assert.Len(t, handles, tt.wantHandles)

			if want := tt.wantDisposeCalls; want > 0 {
				stub, ok := returnHandle.(*jsHandleStub)
				require.True(t, ok)
				assert.Equal(t, want, stub.disposeCalls)
			}
		})
	}

	t.Run("eval_call", func(t *testing.T) {
		t.Parallel()

		const selector = "body"

		_, _ = (&ElementHandle{}).queryAll(
			selector,
			func(_ context.Context, opts evalOptions, jsFunc string, args ...any) (any, error) {
				assert.Equal(t, js.QueryAll, jsFunc)

				assert.Equal(t, opts.forceCallable, true)
				assert.Equal(t, opts.returnByValue, false)

				assert.NotEmpty(t, args)
				assert.IsType(t, args[0], &Selector{})
				sel, ok := args[0].(*Selector)
				require.True(t, ok)
				assert.Equal(t, sel.Selector, selector)

				return nil, nil //nolint:nilnil
			},
		)
	})
}

type jsHandleStub struct {
	JSHandleAPI

	disposeCalls int

	asElementFn     func() *ElementHandle
	getPropertiesFn func() (map[string]JSHandleAPI, error)
}

func (s *jsHandleStub) AsElement() *ElementHandle {
	if s.asElementFn == nil {
		return nil
	}
	return s.asElementFn()
}

func (s *jsHandleStub) Dispose() error {
	s.disposeCalls++
	return nil
}

func (s *jsHandleStub) GetProperties() (map[string]JSHandleAPI, error) {
	if s.getPropertiesFn == nil {
		return nil, nil //nolint:nilnil
	}
	return s.getPropertiesFn()
}
