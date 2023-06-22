package insights

import (
	"time"
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
