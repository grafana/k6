package insights

import (
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

type RequestMetadata struct {
	TraceID        string
	Start          time.Time
	End            time.Time
	TestRunLabels  TestRunLabels
	ProtocolLabels ProtocolLabels
}

type RequestMetadatas []RequestMetadata
