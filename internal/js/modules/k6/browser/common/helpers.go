package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
)

func convertBaseJSHandleTypes(
	_ context.Context, execCtx *ExecutionContext, objHandle *BaseJSHandle,
) (*cdpruntime.CallArgument, error) {
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

func convertArgument(
	ctx context.Context, execCtx *ExecutionContext, arg any,
) (*cdpruntime.CallArgument, error) {
	if escapesSobekValues(arg) {
		return nil, errors.New("sobek.Value escaped")
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
				unserVal = "NaN"
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
		return &cdpruntime.CallArgument{Value: b}, err //nolint:wrapcheck
	}
}

func call(
	ctx context.Context, fn func(context.Context, chan any, chan error), timeout time.Duration,
) (any, error) {
	var (
		result   any
		err      error
		cancelFn context.CancelFunc
		resultCh = make(chan any)
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

//nolint:gocognit
func createWaitForEventHandler(
	ctx context.Context,
	emitter EventEmitter, events []string,
	predicateFn func(data any) bool,
) (
	chan any, context.CancelFunc,
) {
	evCancelCtx, evCancelFn := context.WithCancel(ctx)
	chEvHandler := make(chan Event)
	ch := make(chan any)

	go func() {
		for {
			select {
			case <-evCancelCtx.Done():
				return
			case ev := <-chEvHandler:
				if stringSliceContains(events, ev.typ) {
					if predicateFn != nil {
						if predicateFn(ev.data) {
							select {
							case ch <- ev.data:
							case <-evCancelCtx.Done():
								return
							}
						}
					} else {
						select {
						case ch <- nil:
						case <-evCancelCtx.Done():
							return
						}
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

// Returns a channel that will block until the predicateFn returns true.
// This is similar to createWaitForEventHandler, except that it doesn't
// stop waiting after the first received matching event.
func createWaitForEventPredicateHandler(
	ctx context.Context, emitter EventEmitter, events []string,
	predicateFn func(data any) bool,
) (
	chan any, context.CancelFunc,
) {
	evCancelCtx, evCancelFn := context.WithCancel(ctx)
	chEvHandler := make(chan Event)
	ch := make(chan any)

	go func() {
		for {
			select {
			case <-evCancelCtx.Done():
				return
			case ev := <-chEvHandler:
				if stringSliceContains(events, ev.typ) &&
					predicateFn != nil && predicateFn(ev.data) {
					select {
					case ch <- ev.data:
						close(ch)
						evCancelFn()
					case <-evCancelCtx.Done():
					}
					return
				}
			}
		}
	}()

	emitter.on(evCancelCtx, events, chEvHandler)

	return ch, evCancelFn
}

// panicOrSlowMo panics if err is not nil, otherwise applies slow motion.
//
//nolint:unused
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

// sobekValueExists returns true if a given value is not nil and exists
// (defined and not null) in the sobek runtime.
func sobekValueExists(v sobek.Value) bool {
	return v != nil && !sobek.IsUndefined(v) && !sobek.IsNull(v)
}

// convert is a helper function to convert any value to a given type.
// underneath, it uses json.Marshal and json.Unmarshal to do the conversion.
func convert[T any](from any, to *T) error {
	buf, err := json.Marshal(from)
	if err != nil {
		return err //nolint:wrapcheck
	}
	return json.Unmarshal(buf, to) //nolint:wrapcheck
}

// TODO:
// remove this temporary helper after ensuring the sobek-free
// business logic works.
func escapesSobekValues(args ...any) bool {
	for _, arg := range args {
		if _, ok := arg.(sobek.Value); ok {
			return true
		}
	}
	return false
}
