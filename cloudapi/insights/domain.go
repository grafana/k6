package insights

import (
	"errors"
	"time"
)

type TestRunLabels struct {
	ID       int64
	Scenario string
	Group    string
}

type ProtocolLabels interface {
	IsProtocolLabels()
}

type ProtocolHTTPLabels struct {
	Url        string
	Method     string
	StatusCode int64
}

func (ProtocolHTTPLabels) IsProtocolLabels() {}

type RequestMetadatas []RequestMetadata

type RequestMetadata struct {
	TraceID        string
	Start          time.Time
	End            time.Time
	TestRunLabels  TestRunLabels
	ProtocolLabels ProtocolLabels
}

var (
	ErrEmptyTraceID                      = errors.New("empty trace id")
	ErrEmptyStartTime                    = errors.New("empty start time")
	ErrEmptyEndTime                      = errors.New("empty end time")
	ErrEmptyTestRunID                    = errors.New("empty test run id")
	ErrEmptyTestRunScenario              = errors.New("empty test run scenario")
	ErrEmptyTestRunGroup                 = errors.New("empty test run group")
	ErrEmptyProtocolLabels               = errors.New("empty protocol labels")
	ErrEmptyProtocolHTTPLabelsUrl        = errors.New("empty protocol http labels url")
	ErrEmptyProtocolHTTPLabelsMethod     = errors.New("empty protocol http labels method")
	ErrEmptyProtocolHTTPLabelsStatusCode = errors.New("empty protocol http labels status code")
)

func (rm RequestMetadata) Valid() error {
	if rm.TraceID == "" {
		return ErrEmptyTraceID
	}

	if rm.Start.IsZero() {
		return ErrEmptyStartTime
	}

	if rm.End.IsZero() {
		return ErrEmptyEndTime
	}

	if rm.TestRunLabels.ID == 0 {
		return ErrEmptyTestRunID
	}

	if rm.TestRunLabels.Scenario == "" {
		return ErrEmptyTestRunScenario
	}

	if rm.TestRunLabels.Group == "" {
		return ErrEmptyTestRunGroup
	}

	switch l := rm.ProtocolLabels.(type) {
	case ProtocolHTTPLabels:
		if l.Url == "" {
			return ErrEmptyProtocolHTTPLabelsUrl
		}

		if l.Method == "" {
			return ErrEmptyProtocolHTTPLabelsMethod
		}

		if l.StatusCode == 0 {
			return ErrEmptyProtocolHTTPLabelsStatusCode
		}
	default:
		return ErrEmptyProtocolLabels
	}

	return nil
}
