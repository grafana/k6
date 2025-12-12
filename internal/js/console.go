package js

import (
	"fmt"
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

const (
	functionLog = "[object Function]"
	circularLog = "[Circular]"
)

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

	obj, isObj := v.(*sobek.Object)
	if !isObj {
		return v.String()
	}
	if obj.ClassName() == "Error" {
		return v.String()
	}

	if isBinaryData(obj) {
		return formatBinaryData(obj)
	}

	return c.traverseValue(obj, make(map[*sobek.Object]bool))
}

// traverseValue recursively traverses a [sobek.Value] and returns a formatted
// string representation. For functions, it returns [functionLog], and for
// circular references, it returns [circularLog].
//
// It prevents circular references by keeping track of seen objects.
func (c console) traverseValue(v sobek.Value, seen map[*sobek.Object]bool) string {
	// Handles null and sparse values in arrays.
	if common.IsNullish(v) {
		return "null"
	}

	// Represent functions as a fixed string.
	if _, isFunc := sobek.AssertFunction(v); isFunc {
		return functionLog
	}

	// Handle non-object values.
	obj, ok := v.(*sobek.Object)
	if !ok {
		return formatPrimitive(v)
	}

	// Prevent circular references.
	if seen[obj] {
		return circularLog
	}
	seen[obj] = true
	defer delete(seen, obj)

	if obj.ClassName() == "Error" {
		return v.String()
	}

	// Check for TypedArray and ArrayBuffer.
	if isBinaryData(obj) {
		return formatBinaryData(obj)
	}

	// Handle arrays element-by-element, recursively.
	if obj.ClassName() == "Array" {
		length := obj.Get("length").ToInteger()
		if length == 0 {
			return "[]"
		}
		elements := make([]string, length)
		for i := range length {
			elements[i] = c.traverseValue(obj.Get(strconv.FormatInt(i, 10)), seen)
		}
		return "[ " + strings.Join(elements, ", ") + " ]"
	}

	keys := obj.Keys()
	// for empty objects and other JS types with no enumerable properties.
	if len(keys) == 0 {
		// Date objects have no enumerable keys but should display as ISO string.
		if obj.ClassName() == "Date" {
			return formatDate(obj)
		}
		return "{}"
	}

	// Handle objects key-by-key, recursively.
	pairs := make([]string, len(keys))
	for i, key := range keys {
		pairs[i] = key + ": " + c.traverseValue(obj.Get(key), seen)
	}
	return "{ " + strings.Join(pairs, ", ") + " }"
}

// formatDate formats a Date object as a quoted ISO string.
func formatDate(obj *sobek.Object) string {
	if toISOString, ok := sobek.AssertFunction(obj.Get("toISOString")); ok {
		if result, err := toISOString(obj); err == nil {
			return fmt.Sprintf("%q", result.String())
		}
	}
	return "{}"
}

// formatPrimitive formats a primitive JS value as a string.
func formatPrimitive(v sobek.Value) string {
	switch v.ExportType().Kind() {
	case reflect.String:
		return fmt.Sprintf("%q", v.String())
	default:
		return v.String()
	}
}

// checks for TypedArray and Array Buffer
func isBinaryData(obj *sobek.Object) bool {
	exportType := obj.ExportType()
	if exportType == nil {
		return false
	}

	if _, ok := obj.Export().(sobek.ArrayBuffer); ok {
		return true
	}

	if exportType.Kind() != reflect.Slice {
		return false
	}
	switch exportType.Elem().Kind() {
	case reflect.Int8, reflect.Uint8,
		reflect.Int16, reflect.Uint16,
		reflect.Int32, reflect.Uint32,
		reflect.Float32, reflect.Float64,
		reflect.Int64, reflect.Uint64:
		return true
	default:
		return false
	}
}

func formatBinaryData(obj *sobek.Object) string {
	// ArrayBuffer
	if ab, ok := obj.Export().(sobek.ArrayBuffer); ok {
		bytes := ab.Bytes()
		hexParts := make([]string, len(bytes))
		for i, b := range bytes {
			hexParts[i] = fmt.Sprintf("%02x", b)
		}
		hexStr := strings.Join(hexParts, " ")
		return fmt.Sprintf("ArrayBuffer { [Uint8Contents]: <%s>, byteLength: %d }", hexStr, len(bytes))
	}

	// Typed Array
	exportType := obj.ExportType()
	if exportType != nil && exportType.Kind() == reflect.Slice {
		typeName := typedArrayName(exportType)
		length := obj.Get("length").ToInteger()
		if length == 0 {
			return fmt.Sprintf("%s(0) []", typeName)
		}
		values := make([]string, length)
		for i := int64(0); i < length; i++ {
			val := obj.Get(strconv.FormatInt(i, 10))
			values[i] = val.String()
		}
		valuesStr := strings.Join(values, ", ")
		return fmt.Sprintf("%s(%d) [ %s ]", typeName, length, valuesStr)
	}

	return obj.String()
}

// Maps Go reflect.Kind -> TypedArray name
func typedArrayName(exportType reflect.Type) string {
	// Note: Can't distinguish Uint8ClampedArray this way
	switch exportType.Elem().Kind() {
	case reflect.Int8:
		return "Int8Array"
	case reflect.Uint8:
		return "Uint8Array"
	case reflect.Int16:
		return "Int16Array"
	case reflect.Uint16:
		return "Uint16Array"
	case reflect.Int32:
		return "Int32Array"
	case reflect.Uint32:
		return "Uint32Array"
	case reflect.Float32:
		return "Float32Array"
	case reflect.Float64:
		return "Float64Array"
	case reflect.Int64:
		return "BigInt64Array"
	case reflect.Uint64:
		return "BigUint64Array"
	default:
		return "TypedArray"
	}
}
