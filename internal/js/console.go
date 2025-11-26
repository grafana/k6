package js

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"unicode/utf16"

	"github.com/grafana/sobek"
	"github.com/sirupsen/logrus"
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

const defaultAssertMsg = "Assertion failed"

// Assert logs an error message if the assertion is false.
// https://console.spec.whatwg.org/#assert
//
// Spec pseudo code:
//
//  1. If condition is true, return.
//  2. Let message be a string without any formatting specifiers indicating generically
//     an assertion failure (such as "Assertion failed").
//  3. If data is empty, append message to data.
//  4. Otherwise:
//  1. Let first be data[0].
//  2. If first is not a String, then prepend message to data.
//  3. Otherwise:
//  1. Let concat be the concatenation of message, ':', SPACE, and first.
//  2. Set data[0] to concat.
//
// 5. Perform Logger("assert", data).
//
// Since logrus doesn't support "assert" level, we log at Error level.
func (c console) Assert(condition bool, data ...sobek.Value) {
	if condition {
		return
	}

	toString := func(s string) sobek.String {
		return sobek.StringFromUTF16(utf16.Encode([]rune(s)))
	}

	msg := defaultAssertMsg

	if len(data) == 0 {
		data = append(data, toString(msg))
	} else {
		first := data[0]

		if sobek.IsString(first) {
			msg += ": " + first.String()
			data[0] = toString(msg)
		} else {
			data = append([]sobek.Value{toString(msg)}, data...)
		}
	}

	c.log(logrus.ErrorLevel, data...)
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

	if sobekObj, isObj := v.(*sobek.Object); isObj {
		if sobekObj.ClassName() == "Error" {
			return v.String()
		}
	}

	mv, ok := v.(json.Marshaler)
	if !ok {
		return v.String()
	}

	b, err := json.Marshal(mv)
	if err != nil {
		return v.String()
	}
	return string(b)
}
