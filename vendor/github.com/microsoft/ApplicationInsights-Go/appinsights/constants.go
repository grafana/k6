package appinsights

// NOTE: This file was automatically generated.

import "github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"

// Type of the metric data measurement.
const (
	Measurement contracts.DataPointType = contracts.Measurement
	Aggregation contracts.DataPointType = contracts.Aggregation
)

// Defines the level of severity for the event.
const (
	Verbose     contracts.SeverityLevel = contracts.Verbose
	Information contracts.SeverityLevel = contracts.Information
	Warning     contracts.SeverityLevel = contracts.Warning
	Error       contracts.SeverityLevel = contracts.Error
	Critical    contracts.SeverityLevel = contracts.Critical
)
