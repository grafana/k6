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
	for i := range args {
		if i > 0 {
			strs.WriteString(" ")
		}
		strs.WriteString(c.valueString(args[i]))
	}
	msg := strs.String()

	switch level {
	case logrus.DebugLevel:
		c.logger.Debug(msg)
	case logrus.InfoLevel:
		c.logger.Info(msg)
	case logrus.WarnLevel:
		c.logger.Warn(msg)
	case logrus.ErrorLevel:
		c.logger.Error(msg)
	default:
		panic("unsupported log level: " + level.String())
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
var errorType = reflect.TypeFor[error]()

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

	var sb strings.Builder
	c.traverseValue(&sb, obj, make(map[*sobek.Object]bool))
	return sb.String()
}

// traverseValue recursively traverses a [sobek.Value] and writes a formatted
// string representation to the provided strings.Builder. For functions, it writes
// [functionLog], and for circular references, it writes [circularLog].
//
// It prevents circular references by keeping track of seen objects.
func (c console) traverseValue(sb *strings.Builder, v sobek.Value, seen map[*sobek.Object]bool) {
	// Handles null and sparse values in arrays.
	if common.IsNullish(v) {
		sb.WriteString("null")
		return
	}

	// Represent functions as a fixed string.
	if _, isFunc := sobek.AssertFunction(v); isFunc {
		sb.WriteString(functionLog)
		return
	}

	// Handle non-object values.
	obj, ok := v.(*sobek.Object)
	if !ok {
		formatPrimitive(sb, v)
		return
	}

	// Prevent circular references.
	if seen[obj] {
		sb.WriteString(circularLog)
		return
	}
	seen[obj] = true
	defer delete(seen, obj)

	if obj.ClassName() == "Error" {
		sb.WriteString(v.String())
		return
	}

	// Check for TypedArray and ArrayBuffer.
	if isBinaryData(obj) {
		formatBinaryData(sb, obj)
		return
	}

	// Handle arrays element-by-element, recursively.
	if obj.ClassName() == "Array" {
		length := obj.Get("length").ToInteger()
		if length == 0 {
			sb.WriteString("[]")
			return
		}
		sb.WriteString("[ ")
		for i := range length {
			if i > 0 {
				sb.WriteString(", ")
			}
			c.traverseValue(sb, obj.Get(strconv.FormatInt(i, 10)), seen)
		}
		sb.WriteString(" ]")
		return
	}

	keys := obj.Keys()
	// for empty objects and other JS types with no enumerable properties.
	if len(keys) == 0 {
		// Date objects have no enumerable keys but should display as ISO string.
		if obj.ClassName() == "Date" {
			formatDate(sb, obj)
			return
		}
		sb.WriteString("{}")
		return
	}

	// Handle objects key-by-key, recursively.
	sb.WriteString("{ ")
	for i, key := range keys {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(key)
		sb.WriteString(": ")
		c.traverseValue(sb, obj.Get(key), seen)
	}
	sb.WriteString(" }")
}

// formatDate writes a Date object as a quoted ISO string to the builder.
func formatDate(sb *strings.Builder, obj *sobek.Object) {
	if toISOString, ok := sobek.AssertFunction(obj.Get("toISOString")); ok {
		if result, err := toISOString(obj); err == nil {
			sb.WriteByte('"')
			sb.WriteString(result.String())
			sb.WriteByte('"')
			return
		}
	}
	sb.WriteString("{}")
}

// formatPrimitive writes a primitive JS value to the builder.
func formatPrimitive(sb *strings.Builder, v sobek.Value) {
	switch v.ExportType().Kind() {
	case reflect.String:
		sb.WriteByte('"')
		sb.WriteString(v.String())
		sb.WriteByte('"')
	default:
		sb.WriteString(v.String())
	}
}

// isBinaryData checks for TypedArray and ArrayBuffer.
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

// formatBinaryData writes the formatted representation of TypedArray or ArrayBuffer to the builder.
func formatBinaryData(sb *strings.Builder, obj *sobek.Object) {
	// ArrayBuffer
	if ab, ok := obj.Export().(sobek.ArrayBuffer); ok {
		bytes := ab.Bytes()
		sb.WriteString("ArrayBuffer { [Uint8Contents]: <")
		for i, b := range bytes {
			if i > 0 {
				sb.WriteByte(' ')
			}
			fmt.Fprintf(sb, "%02x", b)
		}
		sb.WriteString(">, byteLength: ")
		sb.WriteString(strconv.Itoa(len(bytes)))
		sb.WriteString(" }")
		return
	}

	// TypedArray
	exportType := obj.ExportType()
	if exportType != nil && exportType.Kind() == reflect.Slice {
		typeName := typedArrayName(exportType)
		length := obj.Get("length").ToInteger()

		sb.WriteString(typeName)
		sb.WriteByte('(')
		sb.WriteString(strconv.FormatInt(length, 10))
		sb.WriteByte(')')

		if length == 0 {
			sb.WriteString(" []")
			return
		}

		sb.WriteString(" [ ")
		for i := range length {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(obj.Get(strconv.FormatInt(i, 10)).String())
		}
		sb.WriteString(" ]")
		return
	}

	sb.WriteString(obj.String())
}

// typedArrayName maps Go reflect.Kind -> TypedArray name.
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
