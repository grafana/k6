package appinsights

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"

	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
)

// Exception telemetry items represent a handled or unhandled exceptions that
// occurred during execution of the monitored application.
type ExceptionTelemetry struct {
	BaseTelemetry
	BaseTelemetryMeasurements

	// Panic message: string, error, or Stringer
	Error interface{}

	// List of stack frames. Use GetCallstack to generate this data.
	Frames []*contracts.StackFrame

	// Severity level.
	SeverityLevel contracts.SeverityLevel
}

// Creates a new exception telemetry item with the specified error and the
// current callstack. This should be used directly from a function that
// handles a recover(), or to report an unexpected error return value from
// a function.
func NewExceptionTelemetry(err interface{}) *ExceptionTelemetry {
	return newExceptionTelemetry(err, 1)
}

func newExceptionTelemetry(err interface{}, skip int) *ExceptionTelemetry {
	return &ExceptionTelemetry{
		Error:         err,
		Frames:        GetCallstack(2 + skip),
		SeverityLevel: Error,
		BaseTelemetry: BaseTelemetry{
			Timestamp:  currentClock.Now(),
			Tags:       make(contracts.ContextTags),
			Properties: make(map[string]string),
		},
		BaseTelemetryMeasurements: BaseTelemetryMeasurements{
			Measurements: make(map[string]float64),
		},
	}
}

func (telem *ExceptionTelemetry) TelemetryData() TelemetryData {
	details := contracts.NewExceptionDetails()
	details.HasFullStack = len(telem.Frames) > 0
	details.ParsedStack = telem.Frames

	if err, ok := telem.Error.(error); ok {
		details.Message = err.Error()
		details.TypeName = reflect.TypeOf(telem.Error).String()
	} else if str, ok := telem.Error.(string); ok {
		details.Message = str
		details.TypeName = "string"
	} else if stringer, ok := telem.Error.(fmt.Stringer); ok {
		details.Message = stringer.String()
		details.TypeName = reflect.TypeOf(telem.Error).String()
	} else if stringer, ok := telem.Error.(fmt.GoStringer); ok {
		details.Message = stringer.GoString()
		details.TypeName = reflect.TypeOf(telem.Error).String()
	} else {
		details.Message = "<unknown>"
		details.TypeName = "<unknown>"
	}

	data := contracts.NewExceptionData()
	data.SeverityLevel = telem.SeverityLevel
	data.Exceptions = []*contracts.ExceptionDetails{details}
	data.Properties = telem.Properties
	data.Measurements = telem.Measurements

	return data
}

// Generates a callstack suitable for inclusion in Application Insights
// exception telemetry for the current goroutine, skipping a number of frames
// specified by skip.
func GetCallstack(skip int) []*contracts.StackFrame {
	var stackFrames []*contracts.StackFrame

	if skip < 0 {
		skip = 0
	}

	stack := make([]uintptr, 64+skip)
	depth := runtime.Callers(skip+1, stack)
	if depth == 0 {
		return stackFrames
	}

	frames := runtime.CallersFrames(stack[:depth])
	level := 0
	for {
		frame, more := frames.Next()

		stackFrame := &contracts.StackFrame{
			Level:    level,
			FileName: frame.File,
			Line:     frame.Line,
		}

		if frame.Function != "" {
			/* Default */
			stackFrame.Method = frame.Function

			/* Break up function into assembly/function */
			lastSlash := strings.LastIndexByte(frame.Function, '/')
			if lastSlash < 0 {
				// e.g. "runtime.gopanic"
				// The below works with lastSlash=0
				lastSlash = 0
			}

			firstDot := strings.IndexByte(frame.Function[lastSlash:], '.')
			if firstDot >= 0 {
				stackFrame.Assembly = frame.Function[:lastSlash+firstDot]
				stackFrame.Method = frame.Function[lastSlash+firstDot+1:]
			}
		}

		stackFrames = append(stackFrames, stackFrame)

		level++
		if !more {
			break
		}
	}

	return stackFrames
}

// Recovers from any active panics and tracks them to the specified
// TelemetryClient.  If rethrow is set to true, then this will panic.
// Should be invoked via defer in functions to monitor.
func TrackPanic(client TelemetryClient, rethrow bool) {
	if r := recover(); r != nil {
		client.Track(newExceptionTelemetry(r, 1))
		if rethrow {
			panic(r)
		}
	}
}
