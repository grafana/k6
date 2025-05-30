package log

// Code generated by cdproto-gen. DO NOT EDIT.

import (
	"fmt"
	"strings"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
)

// Entry log entry.
//
// See: https://chromedevtools.github.io/devtools-protocol/tot/Log#type-LogEntry
type Entry struct {
	Source           Source                  `json:"source"` // Log entry source.
	Level            Level                   `json:"level"`  // Log entry severity.
	Text             string                  `json:"text"`   // Logged text.
	Category         EntryCategory           `json:"category,omitempty,omitzero"`
	Timestamp        *runtime.Timestamp      `json:"timestamp"`                           // Timestamp when this entry was added.
	URL              string                  `json:"url,omitempty,omitzero"`              // URL of the resource if known.
	LineNumber       int64                   `json:"lineNumber,omitempty,omitzero"`       // Line number in the resource.
	StackTrace       *runtime.StackTrace     `json:"stackTrace,omitempty,omitzero"`       // JavaScript stack trace.
	NetworkRequestID network.RequestID       `json:"networkRequestId,omitempty,omitzero"` // Identifier of the network request associated with this entry.
	WorkerID         string                  `json:"workerId,omitempty,omitzero"`         // Identifier of the worker associated with this entry.
	Args             []*runtime.RemoteObject `json:"args,omitempty,omitzero"`             // Call arguments.
}

// ViolationSetting violation configuration setting.
//
// See: https://chromedevtools.github.io/devtools-protocol/tot/Log#type-ViolationSetting
type ViolationSetting struct {
	Name      Violation `json:"name"`      // Violation type.
	Threshold float64   `json:"threshold"` // Time threshold to trigger upon.
}

// Source log entry source.
//
// See: https://chromedevtools.github.io/devtools-protocol/tot/Log#type-LogEntry
type Source string

// String returns the Source as string value.
func (t Source) String() string {
	return string(t)
}

// Source values.
const (
	SourceXML            Source = "xml"
	SourceJavascript     Source = "javascript"
	SourceNetwork        Source = "network"
	SourceStorage        Source = "storage"
	SourceAppcache       Source = "appcache"
	SourceRendering      Source = "rendering"
	SourceSecurity       Source = "security"
	SourceDeprecation    Source = "deprecation"
	SourceWorker         Source = "worker"
	SourceViolation      Source = "violation"
	SourceIntervention   Source = "intervention"
	SourceRecommendation Source = "recommendation"
	SourceOther          Source = "other"
)

// UnmarshalJSON satisfies [json.Unmarshaler].
func (t *Source) UnmarshalJSON(buf []byte) error {
	s := string(buf)
	s = strings.TrimSuffix(strings.TrimPrefix(s, `"`), `"`)

	switch Source(s) {
	case SourceXML:
		*t = SourceXML
	case SourceJavascript:
		*t = SourceJavascript
	case SourceNetwork:
		*t = SourceNetwork
	case SourceStorage:
		*t = SourceStorage
	case SourceAppcache:
		*t = SourceAppcache
	case SourceRendering:
		*t = SourceRendering
	case SourceSecurity:
		*t = SourceSecurity
	case SourceDeprecation:
		*t = SourceDeprecation
	case SourceWorker:
		*t = SourceWorker
	case SourceViolation:
		*t = SourceViolation
	case SourceIntervention:
		*t = SourceIntervention
	case SourceRecommendation:
		*t = SourceRecommendation
	case SourceOther:
		*t = SourceOther
	default:
		return fmt.Errorf("unknown Source value: %v", s)
	}
	return nil
}

// Level log entry severity.
//
// See: https://chromedevtools.github.io/devtools-protocol/tot/Log#type-LogEntry
type Level string

// String returns the Level as string value.
func (t Level) String() string {
	return string(t)
}

// Level values.
const (
	LevelVerbose Level = "verbose"
	LevelInfo    Level = "info"
	LevelWarning Level = "warning"
	LevelError   Level = "error"
)

// UnmarshalJSON satisfies [json.Unmarshaler].
func (t *Level) UnmarshalJSON(buf []byte) error {
	s := string(buf)
	s = strings.TrimSuffix(strings.TrimPrefix(s, `"`), `"`)

	switch Level(s) {
	case LevelVerbose:
		*t = LevelVerbose
	case LevelInfo:
		*t = LevelInfo
	case LevelWarning:
		*t = LevelWarning
	case LevelError:
		*t = LevelError
	default:
		return fmt.Errorf("unknown Level value: %v", s)
	}
	return nil
}

// EntryCategory [no description].
//
// See: https://chromedevtools.github.io/devtools-protocol/tot/Log#type-LogEntry
type EntryCategory string

// String returns the EntryCategory as string value.
func (t EntryCategory) String() string {
	return string(t)
}

// EntryCategory values.
const (
	EntryCategoryCors EntryCategory = "cors"
)

// UnmarshalJSON satisfies [json.Unmarshaler].
func (t *EntryCategory) UnmarshalJSON(buf []byte) error {
	s := string(buf)
	s = strings.TrimSuffix(strings.TrimPrefix(s, `"`), `"`)

	switch EntryCategory(s) {
	case EntryCategoryCors:
		*t = EntryCategoryCors
	default:
		return fmt.Errorf("unknown EntryCategory value: %v", s)
	}
	return nil
}

// Violation violation type.
//
// See: https://chromedevtools.github.io/devtools-protocol/tot/Log#type-ViolationSetting
type Violation string

// String returns the Violation as string value.
func (t Violation) String() string {
	return string(t)
}

// Violation values.
const (
	ViolationLongTask          Violation = "longTask"
	ViolationLongLayout        Violation = "longLayout"
	ViolationBlockedEvent      Violation = "blockedEvent"
	ViolationBlockedParser     Violation = "blockedParser"
	ViolationDiscouragedAPIUse Violation = "discouragedAPIUse"
	ViolationHandler           Violation = "handler"
	ViolationRecurringHandler  Violation = "recurringHandler"
)

// UnmarshalJSON satisfies [json.Unmarshaler].
func (t *Violation) UnmarshalJSON(buf []byte) error {
	s := string(buf)
	s = strings.TrimSuffix(strings.TrimPrefix(s, `"`), `"`)

	switch Violation(s) {
	case ViolationLongTask:
		*t = ViolationLongTask
	case ViolationLongLayout:
		*t = ViolationLongLayout
	case ViolationBlockedEvent:
		*t = ViolationBlockedEvent
	case ViolationBlockedParser:
		*t = ViolationBlockedParser
	case ViolationDiscouragedAPIUse:
		*t = ViolationDiscouragedAPIUse
	case ViolationHandler:
		*t = ViolationHandler
	case ViolationRecurringHandler:
		*t = ViolationRecurringHandler
	default:
		return fmt.Errorf("unknown Violation value: %v", s)
	}
	return nil
}
