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
	"errors"
	"fmt"
	"math"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/dop251/goja"
	"github.com/fatih/color"
	k6common "go.k6.io/k6/js/common"
)

var debugMu = sync.Mutex{}
var debugLastCall int64 = 0

func debugLog(msg string, args ...interface{}) {
	debugMu.Lock()
	defer debugMu.Unlock()
	now := time.Now().UnixNano() / 1000000
	elapsed := now - debugLastCall
	if now == elapsed {
		elapsed = 0
	}
	magenta := color.New(color.FgMagenta).SprintFunc()
	args = []interface{}{fmt.Sprintf("[%d] %s", goRoutineID(), fmt.Sprintf(msg, args...))}
	args = append(args, fmt.Sprintf("%s ms", magenta(elapsed)))
	fmt.Println(args...)
	debugLastCall = now
}

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

func convertArgument(ctx context.Context, execCtx *ExecutionContext, arg goja.Value) (*cdpruntime.CallArgument, error) {
	switch arg.ExportType() {
	case reflect.TypeOf(int64(0)):
		if arg.ToInteger() > math.MaxInt32 {
			return &cdpruntime.CallArgument{
				UnserializableValue: cdpruntime.UnserializableValue(fmt.Sprintf("%dn", arg.ToInteger())),
			}, nil
		}
		b, err := json.Marshal(arg.ToInteger())
		return &cdpruntime.CallArgument{Value: b}, err
	case reflect.TypeOf(float64(0)):
		f := arg.ToFloat()
		if f == math.Float64frombits(0|(1<<63)) {
			return &cdpruntime.CallArgument{
				UnserializableValue: cdpruntime.UnserializableValue("-0"),
			}, nil
		} else if f == math.Inf(0) {
			return &cdpruntime.CallArgument{
				UnserializableValue: cdpruntime.UnserializableValue("Infinity"),
			}, nil
		} else if f == math.Inf(-1) {
			return &cdpruntime.CallArgument{
				UnserializableValue: cdpruntime.UnserializableValue("-Infinity"),
			}, nil
		} else if math.IsNaN(f) {
			return &cdpruntime.CallArgument{
				UnserializableValue: cdpruntime.UnserializableValue("NaN"),
			}, nil
		}
		b, err := json.Marshal(f)
		return &cdpruntime.CallArgument{Value: b}, err
	case reflect.TypeOf(&ElementHandle{}):
		objHandle := arg.Export().(*ElementHandle)
		return convertBaseJSHandleTypes(ctx, execCtx, &objHandle.BaseJSHandle)
	case reflect.TypeOf(&BaseJSHandle{}):
		objHandle := arg.Export().(*BaseJSHandle)
		return convertBaseJSHandleTypes(ctx, execCtx, objHandle)

	}
	b, err := json.Marshal(arg.Export())
	return &cdpruntime.CallArgument{Value: b}, err
}

func callApiWithTimeout(ctx context.Context, fn func(context.Context, chan interface{}, chan error), timeout time.Duration) (interface{}, error) {
	var result interface{}
	var err error
	var cancelFn context.CancelFunc
	resultCh := make(chan interface{})
	errCh := make(chan error)

	apiCtx := ctx
	if timeout > 0 {
		apiCtx, cancelFn = context.WithTimeout(ctx, timeout)
		defer cancelFn()
	}

	go fn(apiCtx, resultCh, errCh)

	select {
	case <-apiCtx.Done():
		if apiCtx.Err() == context.Canceled {
			err = ErrTimedOut
		}
	case result = <-resultCh:
	case err = <-errCh:
	}

	return result, err
}

func errorFromDOMError(domErr string) error {
	switch domErr {
	case "error:notconnected":
		return errors.New("element is not attached to the DOM")
	case "error:notelement":
		return errors.New("node is not an element")
	case "error:nothtmlelement":
		return errors.New("not an HTMLElement")
	case "error:notfillableelement":
		return errors.New("element is not an <input>, <textarea> or [contenteditable] element")
	case "error:notfillableinputtype":
		return errors.New("input of this type cannot be filled")
	case "error:notfillablenumberinput":
		return errors.New("cannot type text into input[type=number]")
	case "error:notvaliddate":
		return errors.New("malformed value")
	case "error:notinput":
		return errors.New("node is not an HTMLInputElement")
	case "error:hasnovalue":
		return errors.New("node is not an HTMLInputElement or HTMLTextAreaElement or HTMLSelectElement")
	case "error:notselect":
		return errors.New("element is not a <select> element")
	case "error:notcheckbox":
		return errors.New("not a checkbox or radio button")
	case "error:notmultiplefileinput":
		return errors.New("non-multiple file input can only accept single file")
	case "error:strictmodeviolation":
		return errors.New("strict mode violation, multiple elements were returned for selector query")
	case "error:notqueryablenode":
		return errors.New("node is not queryable")
	case "error:nthnocapture":
		return errors.New("can't query n-th element in a chained selector with capture")
	case "error:timeout":
		return ErrTimedOut
	default:
		if strings.HasPrefix(domErr, "error:expectednode:") {
			i := strings.LastIndex(domErr, ":")
			return fmt.Errorf("expected node but got %s", domErr[i:])
		}
	}
	return fmt.Errorf(domErr)
}

func goRoutineID() int {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	idField := strings.Fields(strings.TrimPrefix(string(buf[:n]), "goroutine "))[0]
	id, err := strconv.Atoi(idField)
	if err != nil {
		panic(fmt.Sprintf("cannot get goroutine id: %v", err))
	}
	return id
}

func interfaceFromRemoteObject(remoteObject *cdpruntime.RemoteObject) (interface{}, error) {
	if remoteObject.ObjectID != "" {
		return nil, ErrUnexpectedRemoteObjectWithID
	}
	if remoteObject.UnserializableValue != "" {
		if remoteObject.Type == "bigint" {
			n, err := strconv.ParseInt(strings.Replace(remoteObject.UnserializableValue.String(), "n", "", -1), 10, 64)
			if err != nil {
				return nil, BigIntParseError{err}
			}
			return n, nil
		}
		switch remoteObject.UnserializableValue.String() {
		case "-0": // To handle +0 divided by negative number
			return math.Float64frombits(0 | (1 << 63)), nil
		case "NaN":
			return math.NaN(), nil
		case "Infinity":
			return math.Inf(0), nil
		case "-Infinity":
			return math.Inf(-1), nil
		default:
			return nil, UnserializableValueError{remoteObject.UnserializableValue}
		}
	}
	if remoteObject.Type == "undefined" {
		return "undefined", nil
	}
	var i interface{}
	err := json.Unmarshal(remoteObject.Value, &i)
	if err != nil {
		return nil, err
	}
	return i, nil
}

func stringSliceContains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func valueFromRemoteObject(ctx context.Context, remoteObject *cdpruntime.RemoteObject) (goja.Value, error) {
	rt := k6common.GetRuntime(ctx)
	i, err := interfaceFromRemoteObject(remoteObject)
	if i == "undefined" {
		return goja.Undefined(), err
	}
	return rt.ToValue(i), err
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
		return nil, ErrTimedOut
	case evData := <-ch:
		return evData, nil
	}

	return nil, nil
}

// k6Throw throws a k6 error, and before throwing the error, it finds the
// browser process from the context and kills it if it still exists.
// TODO: test
func k6Throw(ctx context.Context, format string, a ...interface{}) {
	rt := k6common.GetRuntime(ctx)
	if rt == nil {
		// this should never happen unless a programmer error
		panic("cannot get k6 runtime")
	}
	defer k6common.Throw(rt, fmt.Errorf(format, a...))

	pid := GetProcessID(ctx)
	if pid == 0 {
		// this should never happen unless a programmer error
		panic("cannot find process id")
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		// optimistically return and don't kill the process
		return
	}
	// no need to check the error for waiting the process to release
	// its resources or whether we could kill it as we're already
	// dying.
	_ = p.Release()
	_ = p.Kill()
}
