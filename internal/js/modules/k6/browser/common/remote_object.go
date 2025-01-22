package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"go.k6.io/k6/internal/js/modules/k6/browser/log"

	"github.com/chromedp/cdproto/runtime"
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

func parseRemoteObjectPreview(op *runtime.ObjectPreview) (map[string]any, error) {
	obj := make(map[string]any)
	var result []error
	if op.Overflow {
		result = append(result, &objectOverflowError{})
	}

	for _, p := range op.Properties {
		val, err := parseRemoteObjectValue(p.Type, p.Subtype, p.Value, p.ValuePreview)
		if err != nil {
			result = append(result, &objectPropertyParseError{err, p.Name})
			continue
		}
		obj[p.Name] = val
	}

	return obj, errors.Join(result...)
}

func parseRemoteObjectValue(
	t runtime.Type, st runtime.Subtype, val string, op *runtime.ObjectPreview,
) (any, error) {
	switch t { //nolint:exhaustive
	case runtime.TypeAccessor:
		return "accessor", nil
	case runtime.TypeBigint:
		n, err := strconv.ParseInt(strings.ReplaceAll(val, "n", ""), 10, 64)
		if err != nil {
			return nil, BigIntParseError{err}
		}
		return n, nil
	case runtime.TypeFunction:
		return "function()", nil
	case runtime.TypeString:
		if !strings.HasPrefix(val, `"`) {
			return val, nil
		}
	case runtime.TypeSymbol:
		return val, nil
	case runtime.TypeObject:
		if op != nil {
			return parseRemoteObjectPreview(op)
		}
		if val == "Object" {
			return val, nil
		}
		if st == runtime.SubtypeNull {
			return nil, nil //nolint:nilnil
		}
	case runtime.TypeUndefined:
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

func parseExceptionDetails(exc *runtime.ExceptionDetails) string {
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
func parseRemoteObject(obj *runtime.RemoteObject) (any, error) {
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

func valueFromRemoteObject(_ context.Context, robj *runtime.RemoteObject) (any, error) {
	val, err := parseRemoteObject(robj)
	if val == "undefined" {
		return nil, err
	}
	return val, err
}

func parseConsoleRemoteObjectPreview(logger *log.Logger, op *runtime.ObjectPreview) (string, error) {
	obj := make(map[string]string)
	if op.Overflow {
		logger.Debugf("parseConsoleRemoteObjectPreview", "object is too large and will be parsed partially")
	}

	for _, p := range op.Properties {
		val, err := parseConsoleRemoteObjectValue(logger, p.Type, p.Subtype, p.Value, p.ValuePreview)
		if err != nil {
			return "", err
		}
		obj[p.Name] = val
	}

	bb, err := json.Marshal(obj)
	if err != nil {
		return "", fmt.Errorf("marshaling object %q to string: %w", obj, err)
	}

	return string(bb), nil
}

func parseConsoleRemoteArrayPreview(logger *log.Logger, op *runtime.ObjectPreview) (string, error) {
	arr := make([]any, 0, len(op.Properties))
	if op.Overflow {
		logger.Debugf("parseConsoleRemoteArrayPreview", "array is too large and will be parsed partially")
	}

	for _, p := range op.Properties {
		val, err := parseConsoleRemoteObjectValue(logger, p.Type, p.Subtype, p.Value, p.ValuePreview)
		if err != nil {
			return "", err
		}
		arr = append(arr, val)
	}

	bb, err := json.Marshal(arr)
	if err != nil {
		return "", fmt.Errorf("marshaling array %q to string: %w", arr, err)
	}

	return string(bb), nil
}

func parseConsoleRemoteObjectValue(
	logger *log.Logger,
	t runtime.Type,
	st runtime.Subtype,
	val string,
	op *runtime.ObjectPreview,
) (string, error) {
	switch t {
	case runtime.TypeAccessor:
		return "accessor", nil
	case runtime.TypeFunction:
		return "function()", nil
	case runtime.TypeString:
		if strings.HasPrefix(val, `"`) {
			val = strings.TrimPrefix(val, `"`)
			val = strings.TrimSuffix(val, `"`)
		}
	case runtime.TypeObject:
		if op != nil {
			if st == "array" {
				return parseConsoleRemoteArrayPreview(logger, op)
			}
			return parseConsoleRemoteObjectPreview(logger, op)
		}
		if val == "Object" {
			return val, nil
		}
		if st == "null" {
			return "null", nil
		}
	case runtime.TypeUndefined:
		return "undefined", nil
	// The following cases are here to clarify that all cases have been
	// considered, but that the result will return val without processing it.
	case runtime.TypeNumber:
	case runtime.TypeBoolean:
	case runtime.TypeSymbol:
	case runtime.TypeBigint:
	}

	return val, nil
}

// parseConsoleRemoteObject is to be used by callers that are working with
// console messages that are written to Chrome's console by the website under
// test.
func parseConsoleRemoteObject(logger *log.Logger, obj *runtime.RemoteObject) (string, error) {
	if obj.UnserializableValue != "" {
		return obj.UnserializableValue.String(), nil
	}

	return parseConsoleRemoteObjectValue(logger, obj.Type, obj.Subtype, string(obj.Value), obj.Preview)
}
