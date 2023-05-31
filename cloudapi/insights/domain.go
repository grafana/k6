package insights

import (
	"errors"
	"time"
)

var (
	errEmptyTraceID                      = errors.New("empty trace id")
	errEmptyStartTime                    = errors.New("empty start time")
	errEmptyEndTime                      = errors.New("empty end time")
	errEmptyTestRunID                    = errors.New("empty test run id")
	errEmptyTestRunScenario              = errors.New("empty test run scenario")
	errEmptyTestRunGroup                 = errors.New("empty test run group")
	errEmptyProtocolLabels               = errors.New("empty protocol labels")
	errEmptyProtocolHTTPLabelsURL        = errors.New("empty protocol http labels url")
	errEmptyProtocolHTTPLabelsMethod     = errors.New("empty protocol http labels method")
	errEmptyProtocolHTTPLabelsStatusCode = errors.New("empty protocol http labels status code")
)

// TestRunLabels describes labels associated with a single test run.
type TestRunLabels struct {
	ID       int64
	Scenario string
	Group    string
}

// ProtocolLabels is a dummy interface that is used for compile-time type checking.
type ProtocolLabels interface {
	IsProtocolLabels()
}

// ProtocolHTTPLabels describes labels associated with a single HTTP request.
type ProtocolHTTPLabels struct {
	URL        string
	Method     string
	StatusCode int64
}

// IsProtocolLabels is a dummy implementation to satisfy the ProtocolLabels interface.
func (ProtocolHTTPLabels) IsProtocolLabels() {
	// Do nothing
}

// RequestMetadatas is a slice of RequestMetadata.
type RequestMetadatas []RequestMetadata

// RequestMetadata describes metadata associated with a single *traced* request.
type RequestMetadata struct {
	TraceID        string
	Start          time.Time
	End            time.Time
	TestRunLabels  TestRunLabels
	ProtocolLabels ProtocolLabels
}

// Valid returns an error if the RequestMetadata is invalid.
// The RequestMetadata is considered invalid if any of the
// fields zero-value or if the ProtocolLabels type is invalid.
func (rm RequestMetadata) Valid() error {
	if rm.TraceID == "" {
		return errEmptyTraceID
	}

	if rm.Start.IsZero() {
		return errEmptyStartTime
	}

	if rm.End.IsZero() {
		return errEmptyEndTime
	}

	if rm.TestRunLabels.ID == 0 {
		return errEmptyTestRunID
	}

	if rm.TestRunLabels.Scenario == "" {
		return errEmptyTestRunScenario
	}

	if rm.TestRunLabels.Group == "" {
		return errEmptyTestRunGroup
	}

	switch l := rm.ProtocolLabels.(type) {
	case ProtocolHTTPLabels:
		if l.URL == "" {
			return errEmptyProtocolHTTPLabelsURL
		}

		if l.Method == "" {
			return errEmptyProtocolHTTPLabelsMethod
		}

		if l.StatusCode == 0 {
			return errEmptyProtocolHTTPLabelsStatusCode
		}
	default:
		return errEmptyProtocolLabels
	}

	return nil
}
