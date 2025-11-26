package js

import (
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/grafana/sobek"
	"github.com/sirupsen/logrus"
	"go.k6.io/k6/js/common"
)

// console represents a JS console implemented as a logrus.FieldLogger.
type console struct {
	logger logrus.FieldLogger
}

// Creates a console with the standard logrus logger.
func newConsole(logger logrus.FieldLogger) *console {
	return &console{logger.WithField("source", "console")}
}

// Creates a console logger with its output set to the file at the provided `filepath`.
func newFileConsole(filepath string, formatter logrus.Formatter, level logrus.Level) (*console, error) {
	//nolint:gosec,forbidigo // see https://github.com/grafana/k6/issues/2565
	f, err := os.OpenFile(filepath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}

	l := logrus.New()
	l.SetLevel(level)
	l.SetOutput(f)
	l.SetFormatter(formatter)

	return &console{l}, nil
}

func (c console) log(level logrus.Level, args ...sobek.Value) {
	var strs strings.Builder
	for i := 0; i < len(args); i++ {
		if i > 0 {
			strs.WriteString(" ")
		}
		strs.WriteString(c.valueString(args[i]))
	}
	msg := strs.String()

	switch level { //nolint:exhaustive
	case logrus.DebugLevel:
		c.logger.Debug(msg)
	case logrus.InfoLevel:
		c.logger.Info(msg)
	case logrus.WarnLevel:
		c.logger.Warn(msg)
	case logrus.ErrorLevel:
		c.logger.Error(msg)
	}
}

func (c console) Log(args ...sobek.Value) {
	c.Info(args...)
}

func (c console) Debug(args ...sobek.Value) {
	c.log(logrus.DebugLevel, args...)
}

func (c console) Info(args ...sobek.Value) {
	c.log(logrus.InfoLevel, args...)
}

func (c console) Warn(args ...sobek.Value) {
	c.log(logrus.WarnLevel, args...)
}

func (c console) Error(args ...sobek.Value) {
	c.log(logrus.ErrorLevel, args...)
}

const functionLog = "[object Function]"

// errorType is used to check if a [sobek.Value] implements the [error] interface.
//
//nolint:gochecknoglobals
var errorType = reflect.TypeOf((*error)(nil)).Elem()

func (c console) valueString(v sobek.Value) string {
	if _, isFunction := sobek.AssertFunction(v); isFunction {
		return functionLog
	}

	if exportType := v.ExportType(); exportType != nil && exportType.Implements(errorType) {
		if exported := v.Export(); exported != nil {
			if err, isError := exported.(error); isError {
				return err.Error()
			}
		}
	}

	var (
		mv any // either marshal from Sobek or Go
		ok bool
	)
	obj, isObj := v.(*sobek.Object)
	if isObj {
		if obj.ClassName() == "Error" {
			return v.String()
		}
		if mv, ok = c.traverseValue(obj, make(map[*sobek.Object]bool)); !ok {
			// We can't marshal circular references.
			return v.String()
		}
	}
	if !isObj {
		if mv, ok = v.(json.Marshaler); !ok {
			return v.String()
		}
	}
	b, err := json.Marshal(mv)
	if err != nil {
		return v.String()
	}

	return string(b)
}

// traverseValue recursively traverses a [sobek.Value], tries to convert it
// into native Go types suitable for JSON marshaling. It returns the converted
// value and a boolean indicating whether the traversal should continue. If a circular
// reference is detected, the boolean is false and the value should not be marshaled.
func (c console) traverseValue(v sobek.Value, seen map[*sobek.Object]bool) (any, bool) {
	// Handles null and sparse values in arrays.
	if common.IsNullish(v) {
		return nil, true
	}

	// Represent functions as a fixed string.
	if _, isFunc := sobek.AssertFunction(v); isFunc {
		return functionLog, true
	}
	// Skip non-object values.
	obj, ok := v.(*sobek.Object)
	if !ok {
		return v, true
	}
	// Prevent circular references.
	if seen[obj] {
		return nil, false
	}
	seen[obj] = true
	defer delete(seen, obj)

	// Handle arrays element-by-element, recursively.
	if obj.ClassName() == "Array" {
		length := obj.Get("length").ToInteger()
		arr := make([]any, length)
		for i := range length {
			val, ok := c.traverseValue(obj.Get(strconv.FormatInt(i, 10)), seen)
			if !ok {
				return nil, false
			}
			arr[i] = val
		}
		return arr, true
	}

	keys := obj.Keys()
	// Fast path for empty objects and other JS types with no enumerable properties.
	if len(keys) == 0 {
		return obj, true
	}
	// Handle objects key-by-key, recursively.
	m := make(map[string]any, len(keys))
	for _, key := range keys {
		val, ok := c.traverseValue(obj.Get(key), seen)
		if !ok {
			return nil, false
		}
		m[key] = val
	}

	return m, true
}
