package testscript

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/grafana/sobek"
)

// console represents a JavaScript console used from go tests.
type console struct {
	tb testing.TB
}

// newConsole creates a new test console.
func newConsole(tb testing.TB) *console {
	tb.Helper()

	return &console{tb: tb}
}

// Assert asserts that the given assertion is true, otherwise logs the provided arguments and fails the test.
// https://console.spec.whatwg.org/#assert
func (c console) Assert(assertion bool, args ...sobek.Value) {
	c.tb.Helper()

	if assertion {
		return
	}

	toString := func(s string) sobek.String {
		return sobek.StringFromUTF16(utf16.Encode([]rune(s)))
	}

	msg := "Assertion failed"

	if len(args) == 0 {
		args = append(args, toString(msg))
	} else {
		first := args[0]

		if sobek.IsString(first) {
			msg += ": " + first.String()
			args[0] = toString(msg)
		} else {
			args = append([]sobek.Value{toString(msg)}, args...)
		}
	}

	c.log("assert", args...)
}

// Log logs the provided arguments at info level.
func (c console) Log(args ...sobek.Value) {
	c.tb.Helper()
	c.Info(args...)
}

// Debug logs the provided arguments at debug level.
func (c console) Debug(args ...sobek.Value) {
	c.tb.Helper()
	c.log("debug", args...)
}

// Info logs the provided arguments at info level.
func (c console) Info(args ...sobek.Value) {
	c.tb.Helper()
	c.log("info", args...)
}

// Warn logs the provided arguments at warn level.
func (c console) Warn(args ...sobek.Value) {
	c.tb.Helper()
	c.log("warn", args...)
}

// Error logs the provided arguments at error level and fails the test.
func (c console) Error(args ...sobek.Value) {
	c.tb.Helper()
	c.log("error", args...)
}

// log logs the provided arguments at the specified level.
// Test fails on "error" and "assert" levels.
func (c console) log(level string, args ...sobek.Value) {
	c.tb.Helper()

	var strs strings.Builder

	for i := range args {
		if i > 0 {
			strs.WriteString(" ")
		}

		strs.WriteString(c.valueString(args[i]))
	}

	msg := strs.String()

	level = strings.ToUpper(level)

	_, err := io.WriteString(c.tb.Output(), fmt.Sprintf("%-6s %s\n", level, msg))
	if err != nil {
		c.tb.Errorf("failed to write to test output: %v", err)
	}

	if level == "ERROR" || level == "ASSERT" {
		c.tb.Fail()
	}
}

const functionLog = "[object Function]"

// errorType is used to check if a [sobek.Value] implements the [error] interface.
//
//nolint:gochecknoglobals
var errorType = reflect.TypeFor[error]()

func (c console) valueString(v sobek.Value) string {
	c.tb.Helper()

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
