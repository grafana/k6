/*
 *
 * xk6-browser - a browser automation extension for k6
 * Copyright (C) 2021 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package common

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/dop251/goja"

	"github.com/grafana/xk6-browser/k6ext"
)

func convertBaseJSHandleTypes(ctx context.Context, execCtx *ExecutionContext, objHandle *BaseJSHandle) (*cdpruntime.CallArgument, error) {
	if objHandle.execCtx != execCtx {
		return nil, ErrWrongExecutionContext
	}
	if objHandle.disposed {
		return nil, ErrJSHandleDisposed
	}
	if objHandle.remoteObject.UnserializableValue.String() != "" {
		return &cdpruntime.CallArgument{
			UnserializableValue: objHandle.remoteObject.UnserializableValue,
		}, nil
	}
	if objHandle.remoteObject.ObjectID.String() == "" {
		return &cdpruntime.CallArgument{Value: objHandle.remoteObject.Value}, nil
	}
	return &cdpruntime.CallArgument{ObjectID: objHandle.remoteObject.ObjectID}, nil
}

// nolint: cyclop
func convertArgument(
	ctx context.Context, execCtx *ExecutionContext, arg interface{},
) (*cdpruntime.CallArgument, error) {
	if gojaVal, ok := arg.(goja.Value); ok {
		arg = gojaVal.Export()
	}
	switch a := arg.(type) {
	case int64:
		if a > math.MaxInt32 {
			return &cdpruntime.CallArgument{
				UnserializableValue: cdpruntime.UnserializableValue(fmt.Sprintf("%dn", a)),
			}, nil
		}
		b, err := json.Marshal(a)
		return &cdpruntime.CallArgument{Value: b}, err
	case float64:
		var unserVal string
		switch a {
		case math.Float64frombits(0 | (1 << 63)):
			unserVal = "-0"
		case math.Inf(0):
			unserVal = "Infinity"
		case math.Inf(-1):
			unserVal = "-Infinity"
		default:
			if math.IsNaN(a) {
				unserVal = "NaN" //nolint: goconst
			}
		}

		if unserVal != "" {
			return &cdpruntime.CallArgument{
				UnserializableValue: cdpruntime.UnserializableValue(unserVal),
			}, nil
		}

		b, err := json.Marshal(a)
		if err != nil {
			err = fmt.Errorf("converting argument '%v': %w", arg, err)
		}

		return &cdpruntime.CallArgument{Value: b}, err
	case *ElementHandle:
		return convertBaseJSHandleTypes(ctx, execCtx, &a.BaseJSHandle)
	case *BaseJSHandle:
		return convertBaseJSHandleTypes(ctx, execCtx, a)
	default:
		b, err := json.Marshal(a)
		return &cdpruntime.CallArgument{Value: b}, err
	}
}

func call(
	ctx context.Context, fn func(context.Context, chan interface{}, chan error), timeout time.Duration,
) (interface{}, error) {
	var (
		result   interface{}
		err      error
		cancelFn context.CancelFunc
		resultCh = make(chan interface{})
		errCh    = make(chan error)
	)
	if timeout > 0 {
		ctx, cancelFn = context.WithTimeout(ctx, timeout)
		defer cancelFn()
	}

	go fn(ctx, resultCh, errCh)

	select {
	case <-ctx.Done():
		err = &k6ext.UserFriendlyError{
			Err:     ctx.Err(),
			Timeout: timeout,
		}
	case result = <-resultCh:
	case err = <-errCh:
	}

	return result, err
}

func stringSliceContains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func createWaitForEventHandler(
	ctx context.Context,
	emitter EventEmitter, events []string,
	predicateFn func(data interface{}) bool,
) (
	chan interface{}, context.CancelFunc,
) {
	evCancelCtx, evCancelFn := context.WithCancel(ctx)
	chEvHandler := make(chan Event)
	ch := make(chan interface{})

	go func() {
		for {
			select {
			case <-evCancelCtx.Done():
				return
			case ev := <-chEvHandler:
				if stringSliceContains(events, ev.typ) {
					if predicateFn != nil {
						if predicateFn(ev.data) {
							ch <- ev.data
						}
					} else {
						ch <- nil
					}
					close(ch)

					// We wait for one matching event only,
					// then remove event handler by cancelling context and stopping goroutine.
					evCancelFn()
					return
				}
			}
		}
	}()

	emitter.on(evCancelCtx, events, chEvHandler)
	return ch, evCancelFn
}

func waitForEvent(ctx context.Context, emitter EventEmitter, events []string, predicateFn func(data interface{}) bool, timeout time.Duration) (interface{}, error) {
	ch, evCancelFn := createWaitForEventHandler(ctx, emitter, events, predicateFn)
	defer evCancelFn() // Remove event handler

	select {
	case <-ctx.Done():
	case <-time.After(timeout):
		return nil, fmt.Errorf("%w after %s", ErrTimedOut, timeout)
	case evData := <-ch:
		return evData, nil
	}

	return nil, nil
}

// panicOrSlowMo panics if err is not nil, otherwise applies slow motion.
func panicOrSlowMo(ctx context.Context, err error) {
	if err != nil {
		k6ext.Panic(ctx, "%w", err)
	}
	applySlowMo(ctx)
}

// TrimQuotes removes surrounding single or double quotes from s.
// We're not using strings.Trim() to avoid trimming unbalanced values,
// e.g. `"'arg` shouldn't change.
// Source: https://stackoverflow.com/a/48451906
func TrimQuotes(s string) string {
	if len(s) >= 2 {
		if c := s[len(s)-1]; s[0] == c && (c == '"' || c == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// gojaValueExists returns true if a given value is not nil and exists
// (defined and not null) in the goja runtime.
func gojaValueExists(v goja.Value) bool {
	return v != nil && !goja.IsUndefined(v) && !goja.IsNull(v)
}

// asGojaValue return v as a goja value.
// panics if v is not a goja value.
func asGojaValue(ctx context.Context, v interface{}) goja.Value {
	gv, ok := v.(goja.Value)
	if !ok {
		k6ext.Panic(ctx, "unexpected type %T", v)
	}
	return gv
}

// gojaValueToString returns v as string.
// panics if v is not a goja value.
func gojaValueToString(ctx context.Context, v interface{}) string {
	return asGojaValue(ctx, v).String()
}
