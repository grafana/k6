package common

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/grafana/xk6-browser/k6ext"
	"github.com/grafana/xk6-browser/log"

	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/dop251/goja"
)

var bigIntRegex = regexp.MustCompile("^[0-9]*n$")

type objectOverflowError struct{}

// Error returns the description of the overflow error.
func (*objectOverflowError) Error() string {
	return "object is too large and will be parsed partially"
}

type objectPropertyParseError struct {
	error
	property string
}

// Error returns the reason of the failure, including the wrapper parsing error
// message.
func (pe *objectPropertyParseError) Error() string {
	return fmt.Sprintf("parsing object property %q: %s", pe.property, pe.error)
}

// Unwrap returns the wrapped parsing error.
func (pe *objectPropertyParseError) Unwrap() error {
	return pe.error
}

type multiError struct {
	Errors []error
}

func (me *multiError) append(err error) {
	me.Errors = append(me.Errors, err)
}

func (me multiError) Error() string {
	if len(me.Errors) == 0 {
		return ""
	}
	if len(me.Errors) == 1 {
		return me.Errors[0].Error()
	}

	var buf strings.Builder
	fmt.Fprintf(&buf, "%d errors occurred:\n", len(me.Errors))
	for _, e := range me.Errors {
		fmt.Fprintf(&buf, "\t* %s\n", e)
	}

	return buf.String()
}

func multierror(err error, errs ...error) error {
	me := &multiError{}
	// We can't use errors.As(), as we want to know if err is of type
	// multiError, not any error in the chain. If err contains a wrapped
	// multierror, start a new multiError that will contain err.
	e, ok := err.(*multiError) //nolint:errorlint

	if ok {
		me = e
	} else if err != nil {
		me.append(err)
	}

	for _, e := range errs {
		me.append(e)
	}

	return me
}

type remoteObjectParseError struct {
	error
	typ     string
	subType string
	val     string
}

// Error returns a string representation of the error.
func (e *remoteObjectParseError) Error() string {
	return fmt.Sprintf("parsing remote object with type: %s subtype: %s val: %s err: %s",
		e.typ, e.subType, e.val, e.error.Error())
}

// Unwrap returns the wrapped parsing error.
func (e *remoteObjectParseError) Unwrap() error {
	return e.error
}

func parseRemoteObjectPreview(op *cdpruntime.ObjectPreview) (map[string]any, error) {
	obj := make(map[string]any)
	var result error
	if op.Overflow {
		result = multierror(result, &objectOverflowError{})
	}

	for _, p := range op.Properties {
		val, err := parseRemoteObjectValue(p.Type, p.Subtype, p.Value, p.ValuePreview)
		if err != nil {
			result = multierror(result, &objectPropertyParseError{err, p.Name})
			continue
		}
		obj[p.Name] = val
	}

	return obj, result
}

//nolint:cyclop
func parseRemoteObjectValue(
	t cdpruntime.Type, st cdpruntime.Subtype, val string, op *cdpruntime.ObjectPreview,
) (any, error) {
	switch t {
	case cdpruntime.TypeAccessor:
		return "accessor", nil
	case cdpruntime.TypeBigint:
		n, err := strconv.ParseInt(strings.Replace(val, "n", "", -1), 10, 64)
		if err != nil {
			return nil, BigIntParseError{err}
		}
		return n, nil
	case cdpruntime.TypeFunction:
		return "function()", nil
	case cdpruntime.TypeString:
		if !strings.HasPrefix(val, `"`) {
			return val, nil
		}
	case cdpruntime.TypeSymbol:
		return val, nil
	case cdpruntime.TypeObject:
		if op != nil {
			return parseRemoteObjectPreview(op)
		}
		if val == "Object" {
			return val, nil
		}
		if st == "null" {
			return "null", nil
		}
	case cdpruntime.TypeUndefined:
		return "undefined", nil
	}

	var v any
	if err := json.Unmarshal([]byte(val), &v); err != nil {
		return nil, &remoteObjectParseError{
			error:   err,
			typ:     string(t),
			subType: string(st),
			val:     val,
		}
	}

	return v, nil
}

func parseExceptionDetails(exc *cdpruntime.ExceptionDetails) string {
	if exc == nil {
		return ""
	}
	var errMsg string
	if exc.Exception != nil {
		errMsg = exc.Exception.Description
		if errMsg == "" {
			if o, _ := parseRemoteObject(exc.Exception); o != nil {
				errMsg = fmt.Sprintf("%s", o)
			}
		}
	}
	return errMsg
}

// parseRemoteObject is to be used by callers that require the string value
// to be parsed to a Go type.
func parseRemoteObject(obj *cdpruntime.RemoteObject) (any, error) {
	uv := obj.UnserializableValue

	if uv == "" {
		return parseRemoteObjectValue(obj.Type, obj.Subtype, string(obj.Value), obj.Preview)
	}

	if bigIntRegex.Match([]byte(uv)) {
		n, err := strconv.ParseInt(strings.ReplaceAll(uv.String(), "n", ""), 10, 64)
		if err != nil {
			return nil, BigIntParseError{err}
		}
		return n, nil
	}

	switch uv.String() {
	case "-0": // To handle +0 divided by negative number
		return math.Float64frombits(0 | (1 << 63)), nil
	case "NaN":
		return math.NaN(), nil
	case "Infinity":
		return math.Inf(0), nil
	case "-Infinity":
		return math.Inf(-1), nil
	}

	// We should never get here, as previous switch statement should
	// be exhaustive and contain all possible unserializable values.
	// See: https://chromedevtools.github.io/devtools-protocol/tot/Runtime/#type-UnserializableValue
	return nil, UnserializableValueError{uv}
}

func valueFromRemoteObject(ctx context.Context, robj *cdpruntime.RemoteObject) (goja.Value, error) {
	val, err := parseRemoteObject(robj)
	if val == "undefined" {
		return goja.Undefined(), err
	}
	return k6ext.Runtime(ctx).ToValue(val), err
}

func parseConsoleRemoteObjectPreview(logger *log.Logger, op *cdpruntime.ObjectPreview) string {
	obj := make(map[string]string)
	if op.Overflow {
		logger.Infof("parseConsoleRemoteObjectPreview", "object is too large and will be parsed partially")
	}

	for _, p := range op.Properties {
		val := parseConsoleRemoteObjectValue(logger, p.Type, p.Subtype, p.Value, p.ValuePreview)
		obj[p.Name] = val
	}

	bb, err := json.Marshal(obj)
	if err != nil {
		logger.Errorf("parseConsoleRemoteObjectPreview", "failed to marshal object to string: %v", err)
	}

	return string(bb)
}

func parseConsoleRemoteArrayPreview(logger *log.Logger, op *cdpruntime.ObjectPreview) string {
	arr := make([]any, 0, len(op.Properties))
	if op.Overflow {
		logger.Warnf("parseConsoleRemoteArrayPreview", "array is too large and will be parsed partially")
	}

	for _, p := range op.Properties {
		val := parseConsoleRemoteObjectValue(logger, p.Type, p.Subtype, p.Value, p.ValuePreview)
		arr = append(arr, val)
	}

	bb, err := json.Marshal(arr)
	if err != nil {
		logger.Errorf("parseConsoleRemoteArrayPreview", "failed to marshal array to string: %v", err)
	}

	return string(bb)
}

//nolint:cyclop
func parseConsoleRemoteObjectValue(
	logger *log.Logger,
	t cdpruntime.Type,
	st cdpruntime.Subtype,
	val string,
	op *cdpruntime.ObjectPreview,
) string {
	switch t {
	case cdpruntime.TypeAccessor:
		return "accessor"
	case cdpruntime.TypeFunction:
		return "function()"
	case cdpruntime.TypeString:
		if strings.HasPrefix(val, `"`) {
			val = strings.TrimPrefix(val, `"`)
			val = strings.TrimSuffix(val, `"`)
		}
	case cdpruntime.TypeObject:
		if op != nil {
			if st == "array" {
				return parseConsoleRemoteArrayPreview(logger, op)
			}
			return parseConsoleRemoteObjectPreview(logger, op)
		}
		if val == "Object" {
			return val
		}
		if st == "null" {
			return "null"
		}
	case cdpruntime.TypeUndefined:
		return "undefined"
	// The following cases are here to clarify that all cases have been
	// considered, but that the result will return val without processing it.
	case cdpruntime.TypeNumber:
	case cdpruntime.TypeBoolean:
	case cdpruntime.TypeSymbol:
	case cdpruntime.TypeBigint:
	}

	return val
}

// parseConsoleRemoteObject is to be used by callers that are working with
// console messages that are written to Chrome's console by the website under
// test.
func parseConsoleRemoteObject(logger *log.Logger, obj *cdpruntime.RemoteObject) string {
	if obj.UnserializableValue != "" {
		return obj.UnserializableValue.String()
	}

	return parseConsoleRemoteObjectValue(logger, obj.Type, obj.Subtype, string(obj.Value), obj.Preview)
}
