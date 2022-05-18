package common

import (
	"context"
	"errors"
	"testing"

	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/common/js"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorFromDOMError(t *testing.T) {
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

//nolint:funlen
func TestQueryAll(t *testing.T) {
	t.Parallel()

	var (
		nilHandle    = func() api.ElementHandle { return nil }
		nonNilHandle = func() api.ElementHandle { return &ElementHandle{} }
	)

	for name, tt := range map[string]struct {
		selector                      string
		returnHandle                  func() interface{}
		returnErr                     error
		wantErr                       bool
		wantHandles, wantDisposeCalls int
	}{
		"invalid_selector": {
			selector:     "*=*>>*=*",
			returnHandle: func() interface{} { return nil },
			returnErr:    errors.New("any"),
			wantErr:      true,
		},
		"cannot_evaluate": {
			selector:     "*",
			returnHandle: func() interface{} { return nil },
			returnErr:    errors.New("any"),
			wantErr:      true,
		},
		"nil_handles_no_err": {
			selector:     "*",
			returnHandle: func() interface{} { return nil },
			returnErr:    nil,
			wantErr:      false,
		},
		"invalid_js_handle": {
			selector:     "*",
			returnHandle: func() interface{} { return "an invalid handle" },
			wantErr:      true,
		},
		"disposes_main_handle": {
			selector:         "*",
			returnHandle:     func() interface{} { return &jsHandleStub{} },
			wantDisposeCalls: 1,
		},
		// disposes the main handle and all its nil children
		"disposes_handles": {
			selector: "*",
			returnHandle: func() interface{} {
				handles := &jsHandleStub{
					asElementFn: nilHandle,
				}
				handles.getPropertiesFn = func() map[string]api.JSHandle {
					return map[string]api.JSHandle{
						"1": handles,
						"2": handles,
					}
				}

				return handles
			},
			wantDisposeCalls: 3,
		},
		// only returns non-nil handles
		"returns_elems": {
			selector: "*",
			returnHandle: func() interface{} {
				childHandles := map[string]api.JSHandle{
					"1": &jsHandleStub{asElementFn: nonNilHandle},
					"2": &jsHandleStub{asElementFn: nonNilHandle},
					"3": &jsHandleStub{asElementFn: nilHandle},
				}
				return &jsHandleStub{
					getPropertiesFn: func() map[string]api.JSHandle {
						return childHandles
					},
				}
			},
			wantHandles: 2,
		},
	} {
		name, tt := name, tt

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var (
				returnHandle = tt.returnHandle()
				evalFunc     = func(_ context.Context, _ evalOptions, _ string, _ ...interface{}) (interface{}, error) {
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
		const selector = "body"

		_, _ = (&ElementHandle{}).queryAll(
			selector,
			func(_ context.Context, opts evalOptions, jsFunc string, args ...interface{}) (interface{}, error) {
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
	api.JSHandle

	disposeCalls int

	asElementFn     func() api.ElementHandle
	getPropertiesFn func() map[string]api.JSHandle
}

func (s *jsHandleStub) AsElement() api.ElementHandle {
	if s.asElementFn == nil {
		return nil
	}
	return s.asElementFn()
}

func (s *jsHandleStub) Dispose() {
	s.disposeCalls++
}

func (s *jsHandleStub) GetProperties() map[string]api.JSHandle {
	if s.getPropertiesFn == nil {
		return nil
	}
	return s.getPropertiesFn()
}
