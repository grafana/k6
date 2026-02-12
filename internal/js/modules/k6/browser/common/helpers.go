package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"

	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"

	"go.k6.io/k6/lib"
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
			Err:     lib.ContextErr(ctx),
			Timeout: timeout,
		}
	case result = <-resultCh:
	case err = <-errCh:
	}

	return result, err
}

//nolint:gocognit
func createWaitForEventHandler(
	ctx context.Context,
	emitter EventEmitter, events []string,
	predicateFn func(data any) bool,
) (
	chan any, context.CancelCauseFunc,
) {
	evCancelCtx, evCancelFn := context.WithCancelCause(ctx)
	chEvHandler := make(chan Event)
	ch := make(chan any)

	go func() {
		for {
			select {
			case <-evCancelCtx.Done():
				return
			case ev := <-chEvHandler:
				if slices.Contains(events, ev.typ) {
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
					// then remove the event handler by cancelling context and stopping goroutine.
					evCancelFn(nil)

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
	chan any, context.CancelCauseFunc,
) {
	evCancelCtx, evCancelFn := context.WithCancelCause(ctx)
	chEvHandler := make(chan Event)
	ch := make(chan any)

	go func() {
		for {
			select {
			case <-evCancelCtx.Done():
				return
			case ev := <-chEvHandler:
				if slices.Contains(events, ev.typ) &&
					predicateFn != nil && predicateFn(ev.data) {
					select {
					case ch <- ev.data:
						close(ch)
						evCancelFn(nil)
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

// patternMatcherFunc is a function that matches a string against a pattern.
type patternMatcherFunc func(string) (bool, error)

// RegExMatcher is a function that matches a pattern against
// a string and returns true if they match, false otherwise.
type RegExMatcher func(pattern, str string) (bool, error)

// newPatternMatcher returns a [patternMatcherFunc] that uses string matching for
// quoted strings, and ECMAScript (Sobek) regex matching for others. If the pattern
// is empty or a single quote, it matches any string.
func newPatternMatcher(pattern string, rm RegExMatcher) (patternMatcherFunc, error) {
	if pattern == "" || pattern == "''" {
		return func(s string) (bool, error) { return true, nil }, nil
	}
	if isQuotedText(pattern) {
		return func(s string) (bool, error) { return "'"+s+"'" == pattern, nil }, nil
	}
	if rm == nil {
		return nil, fmt.Errorf("regex matcher must be provided")
	}
	return func(s string) (bool, error) {
		ok, err := rm(pattern, s)
		if err != nil {
			return false, fmt.Errorf("matching %q against pattern %q: %w", s, pattern, err)
		}
		return ok, nil
	}, nil
}

// Match evaluates the supplied string against the given pattern using the provided
// [RegExMatcher] for regex patterns. The matcher behavior is determined by [newPatternMatcher].
func (rm RegExMatcher) Match(pattern, s string) (bool, error) {
	m, err := newPatternMatcher(pattern, rm)
	if err != nil {
		return false, fmt.Errorf("matching %q against pattern %q: %w", s, pattern, err)
	}
	return m(s)
}

var sourceURLRegex = regexp.MustCompile(`(?s)[\040\t]*//[@#] sourceURL=\s*(\S*?)\s*$`)

func hasSourceURL(js string) bool {
	lastNotEmptyIndex := strings.LastIndexFunc(js, func(r rune) bool {
		switch r {
		case '\n', '\r', ' ', '\t':
			return false
		default:
			return true
		}
	})
	lastNewLineBeforeLastLineIndex := 0 // default to going through the whole string
	if lastNotEmptyIndex != -1 {        // if there are no new lines - go through the whole string
		lastNewLineBeforeLastLineIndex = strings.LastIndex(js[:lastNotEmptyIndex], "\n")
		if lastNewLineBeforeLastLineIndex == -1 { // reset to zero again
			lastNewLineBeforeLastLineIndex = 0
		}
	}

	return sourceURLRegex.MatchString(js[lastNewLineBeforeLastLineIndex:])
}
