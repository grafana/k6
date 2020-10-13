package appinsights

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
)

// Common interface implemented by telemetry data contracts
type TelemetryData interface {
	EnvelopeName(string) string
	BaseType() string
	Sanitize() []string
}

// Common interface implemented by telemetry items that can be passed to
// TelemetryClient.Track
type Telemetry interface {
	// Gets the time when this item was measured
	Time() time.Time

	// Sets the timestamp to the specified time.
	SetTime(time.Time)

	// Gets context data containing extra, optional tags.  Overrides
	// values found on client TelemetryContext.
	ContextTags() map[string]string

	// Gets the data contract as it will be submitted to the data
	// collector.
	TelemetryData() TelemetryData

	// Gets custom properties to submit with the telemetry item.
	GetProperties() map[string]string

	// Gets custom measurements to submit with the telemetry item.
	GetMeasurements() map[string]float64
}

// BaseTelemetry is the common base struct for telemetry items.
type BaseTelemetry struct {
	// The time this when this item was measured
	Timestamp time.Time

	// Custom properties
	Properties map[string]string

	// Telemetry Context containing extra, optional tags.
	Tags contracts.ContextTags
}

// BaseTelemetryMeasurements provides the Measurements field for telemetry
// items that support it.
type BaseTelemetryMeasurements struct {
	// Custom measurements
	Measurements map[string]float64
}

// BaseTelemetryNoMeasurements provides no Measurements field for telemetry
// items that omit it.
type BaseTelemetryNoMeasurements struct {
}

// Time returns the timestamp when this was measured.
func (item *BaseTelemetry) Time() time.Time {
	return item.Timestamp
}

// SetTime sets the timestamp to the specified time.
func (item *BaseTelemetry) SetTime(t time.Time) {
	item.Timestamp = t
}

// Gets context data containing extra, optional tags.  Overrides values
// found on client TelemetryContext.
func (item *BaseTelemetry) ContextTags() map[string]string {
	return item.Tags
}

// Gets custom properties to submit with the telemetry item.
func (item *BaseTelemetry) GetProperties() map[string]string {
	return item.Properties
}

// Gets custom measurements to submit with the telemetry item.
func (item *BaseTelemetryMeasurements) GetMeasurements() map[string]float64 {
	return item.Measurements
}

// GetMeasurements returns nil for telemetry items that do not support measurements.
func (item *BaseTelemetryNoMeasurements) GetMeasurements() map[string]float64 {
	return nil
}

// Trace telemetry items represent printf-like trace statements that can be
// text searched.
type TraceTelemetry struct {
	BaseTelemetry
	BaseTelemetryNoMeasurements

	// Trace message
	Message string

	// Severity level
	SeverityLevel contracts.SeverityLevel
}

// Creates a trace telemetry item with the specified message and severity
// level.
func NewTraceTelemetry(message string, severityLevel contracts.SeverityLevel) *TraceTelemetry {
	return &TraceTelemetry{
		Message:       message,
		SeverityLevel: severityLevel,
		BaseTelemetry: BaseTelemetry{
			Timestamp:  currentClock.Now(),
			Tags:       make(contracts.ContextTags),
			Properties: make(map[string]string),
		},
	}
}

func (trace *TraceTelemetry) TelemetryData() TelemetryData {
	data := contracts.NewMessageData()
	data.Message = trace.Message
	data.Properties = trace.Properties
	data.SeverityLevel = trace.SeverityLevel

	return data
}

// Event telemetry items represent structured event records.
type EventTelemetry struct {
	BaseTelemetry
	BaseTelemetryMeasurements

	// Event name
	Name string
}

// Creates an event telemetry item with the specified name.
func NewEventTelemetry(name string) *EventTelemetry {
	return &EventTelemetry{
		Name: name,
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

func (event *EventTelemetry) TelemetryData() TelemetryData {
	data := contracts.NewEventData()
	data.Name = event.Name
	data.Properties = event.Properties
	data.Measurements = event.Measurements

	return data
}

// Metric telemetry items each represent a single data point.
type MetricTelemetry struct {
	BaseTelemetry
	BaseTelemetryNoMeasurements

	// Metric name
	Name string

	// Sampled value
	Value float64
}

// Creates a metric telemetry sample with the specified name and value.
func NewMetricTelemetry(name string, value float64) *MetricTelemetry {
	return &MetricTelemetry{
		Name:  name,
		Value: value,
		BaseTelemetry: BaseTelemetry{
			Timestamp:  currentClock.Now(),
			Tags:       make(contracts.ContextTags),
			Properties: make(map[string]string),
		},
	}
}

func (metric *MetricTelemetry) TelemetryData() TelemetryData {
	dataPoint := contracts.NewDataPoint()
	dataPoint.Name = metric.Name
	dataPoint.Value = metric.Value
	dataPoint.Count = 1
	dataPoint.Kind = contracts.Measurement

	data := contracts.NewMetricData()
	data.Metrics = []*contracts.DataPoint{dataPoint}
	data.Properties = metric.Properties

	return data
}

// Aggregated metric telemetry items represent an aggregation of data points
// over time. These values can be calculated by the caller or with the AddData
// function.
type AggregateMetricTelemetry struct {
	BaseTelemetry
	BaseTelemetryNoMeasurements

	// Metric name
	Name string

	// Sum of individual measurements
	Value float64

	// Minimum value of the aggregated metric
	Min float64

	// Maximum value of the aggregated metric
	Max float64

	// Count of measurements in the sample
	Count int

	// Standard deviation of the aggregated metric
	StdDev float64

	// Variance of the aggregated metric.  As an invariant,
	// either this or the StdDev should be zero at any given time.
	// If both are non-zero then StdDev takes precedence.
	Variance float64
}

// Creates a new aggregated metric telemetry item with the specified name.
// Values should be set on the object returned before submission.
func NewAggregateMetricTelemetry(name string) *AggregateMetricTelemetry {
	return &AggregateMetricTelemetry{
		Name:  name,
		Count: 0,
		BaseTelemetry: BaseTelemetry{
			Timestamp:  currentClock.Now(),
			Tags:       make(contracts.ContextTags),
			Properties: make(map[string]string),
		},
	}
}

// Adds data points to the aggregate totals included in this telemetry item.
// This can be used for all the data at once or incrementally.  Calculates
// Min, Max, Sum, Count, and StdDev (by way of Variance).
func (agg *AggregateMetricTelemetry) AddData(values []float64) {
	if agg.StdDev != 0.0 {
		// If StdDev is non-zero, then square it to produce
		// the variance, which is better for incremental calculations,
		// and then zero it out.
		agg.Variance = agg.StdDev * agg.StdDev
		agg.StdDev = 0.0
	}

	vsum := agg.addData(values, agg.Variance*float64(agg.Count))
	if agg.Count > 0 {
		agg.Variance = vsum / float64(agg.Count)
	}
}

// Adds sampled data points to the aggregate totals included in this telemetry item.
// This can be used for all the data at once or incrementally.  Differs from AddData
// in how it calculates standard deviation, and should not be used interchangeably
// with AddData.
func (agg *AggregateMetricTelemetry) AddSampledData(values []float64) {
	if agg.StdDev != 0.0 {
		// If StdDev is non-zero, then square it to produce
		// the variance, which is better for incremental calculations,
		// and then zero it out.
		agg.Variance = agg.StdDev * agg.StdDev
		agg.StdDev = 0.0
	}

	vsum := agg.addData(values, agg.Variance*float64(agg.Count-1))
	if agg.Count > 1 {
		// Sampled values should divide by n-1
		agg.Variance = vsum / float64(agg.Count-1)
	}
}

func (agg *AggregateMetricTelemetry) addData(values []float64, vsum float64) float64 {
	if len(values) == 0 {
		return vsum
	}

	// Running tally of the mean is important for incremental variance computation.
	var mean float64

	if agg.Count == 0 {
		agg.Min = values[0]
		agg.Max = values[0]
	} else {
		mean = agg.Value / float64(agg.Count)
	}

	for _, x := range values {
		// Update Min, Max, Count, and Value
		agg.Count++
		agg.Value += x

		if x < agg.Min {
			agg.Min = x
		}

		if x > agg.Max {
			agg.Max = x
		}

		// Welford's algorithm to compute variance.  The divide occurs in the caller.
		newMean := agg.Value / float64(agg.Count)
		vsum += (x - mean) * (x - newMean)
		mean = newMean
	}

	return vsum
}

func (agg *AggregateMetricTelemetry) TelemetryData() TelemetryData {
	dataPoint := contracts.NewDataPoint()
	dataPoint.Name = agg.Name
	dataPoint.Value = agg.Value
	dataPoint.Kind = contracts.Aggregation
	dataPoint.Min = agg.Min
	dataPoint.Max = agg.Max
	dataPoint.Count = agg.Count

	if agg.StdDev != 0.0 {
		dataPoint.StdDev = agg.StdDev
	} else if agg.Variance > 0.0 {
		dataPoint.StdDev = math.Sqrt(agg.Variance)
	}

	data := contracts.NewMetricData()
	data.Metrics = []*contracts.DataPoint{dataPoint}
	data.Properties = agg.Properties

	return data
}

// Request telemetry items represents completion of an external request to the
// application and contains a summary of that request execution and results.
type RequestTelemetry struct {
	BaseTelemetry
	BaseTelemetryMeasurements

	// Identifier of a request call instance. Used for correlation between request
	// and other telemetry items.
	Id string

	// Request name. For HTTP requests it represents the HTTP method and URL path template.
	Name string

	// URL of the request with all query string parameters.
	Url string

	// Duration to serve the request.
	Duration time.Duration

	// Results of a request execution. HTTP status code for HTTP requests.
	ResponseCode string

	// Indication of successful or unsuccessful call.
	Success bool

	// Source of the request. Examplese are the instrumentation key of the caller
	// or the ip address of the caller.
	Source string
}

// Creates a new request telemetry item for HTTP requests. The success value will be
// computed from responseCode, and the timestamp will be set to the current time minus
// the duration.
func NewRequestTelemetry(method, uri string, duration time.Duration, responseCode string) *RequestTelemetry {
	success := true
	code, err := strconv.Atoi(responseCode)
	if err == nil {
		success = code < 400 || code == 401
	}

	nameUri := uri

	// Sanitize URL for the request name
	if parsedUrl, err := url.Parse(uri); err == nil {
		// Remove the query
		parsedUrl.RawQuery = ""
		parsedUrl.ForceQuery = false

		// Remove the fragment
		parsedUrl.Fragment = ""

		// Remove the user info, if any.
		parsedUrl.User = nil

		// Write back to name
		nameUri = parsedUrl.String()
	}

	return &RequestTelemetry{
		Name:         fmt.Sprintf("%s %s", method, nameUri),
		Url:          uri,
		Id:           newUUID().String(),
		Duration:     duration,
		ResponseCode: responseCode,
		Success:      success,
		BaseTelemetry: BaseTelemetry{
			Timestamp:  currentClock.Now().Add(-duration),
			Tags:       make(contracts.ContextTags),
			Properties: make(map[string]string),
		},
		BaseTelemetryMeasurements: BaseTelemetryMeasurements{
			Measurements: make(map[string]float64),
		},
	}
}

// Sets the timestamp and duration of this telemetry item based on the provided
// start and end times.
func (request *RequestTelemetry) MarkTime(startTime, endTime time.Time) {
	request.Timestamp = startTime
	request.Duration = endTime.Sub(startTime)
}

func (request *RequestTelemetry) TelemetryData() TelemetryData {
	data := contracts.NewRequestData()
	data.Name = request.Name
	data.Duration = formatDuration(request.Duration)
	data.ResponseCode = request.ResponseCode
	data.Success = request.Success
	data.Url = request.Url
	data.Source = request.Source

	if request.Id == "" {
		data.Id = newUUID().String()
	} else {
		data.Id = request.Id
	}

	data.Properties = request.Properties
	data.Measurements = request.Measurements
	return data
}

// Remote dependency telemetry items represent interactions of the monitored
// component with a remote component/service like SQL or an HTTP endpoint.
type RemoteDependencyTelemetry struct {
	BaseTelemetry
	BaseTelemetryMeasurements

	// Name of the command that initiated this dependency call. Low cardinality
	// value. Examples are stored procedure name and URL path template.
	Name string

	// Identifier of a dependency call instance. Used for correlation with the
	// request telemetry item corresponding to this dependency call.
	Id string

	// Result code of a dependency call. Examples are SQL error code and HTTP
	// status code.
	ResultCode string

	// Duration of the remote call.
	Duration time.Duration

	// Indication of successful or unsuccessful call.
	Success bool

	// Command initiated by this dependency call. Examples are SQL statement and
	// HTTP URL's with all the query parameters.
	Data string

	// Dependency type name. Very low cardinality. Examples are SQL, Azure table,
	// and HTTP.
	Type string

	// Target site of a dependency call. Examples are server name, host address.
	Target string
}

// Builds a new Remote Dependency telemetry item, with the specified name,
// dependency type, target site, and success status.
func NewRemoteDependencyTelemetry(name, dependencyType, target string, success bool) *RemoteDependencyTelemetry {
	return &RemoteDependencyTelemetry{
		Name:    name,
		Type:    dependencyType,
		Target:  target,
		Success: success,
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

// Sets the timestamp and duration of this telemetry item based on the provided
// start and end times.
func (telem *RemoteDependencyTelemetry) MarkTime(startTime, endTime time.Time) {
	telem.Timestamp = startTime
	telem.Duration = endTime.Sub(startTime)
}

func (telem *RemoteDependencyTelemetry) TelemetryData() TelemetryData {
	data := contracts.NewRemoteDependencyData()
	data.Name = telem.Name
	data.Id = telem.Id
	data.ResultCode = telem.ResultCode
	data.Duration = formatDuration(telem.Duration)
	data.Success = telem.Success
	data.Data = telem.Data
	data.Target = telem.Target
	data.Properties = telem.Properties
	data.Measurements = telem.Measurements
	data.Type = telem.Type

	return data
}

// Avaibility telemetry items represent the result of executing an availability
// test.
type AvailabilityTelemetry struct {
	BaseTelemetry
	BaseTelemetryMeasurements

	// Identifier of a test run. Used to correlate steps of test run and
	// telemetry generated by the service.
	Id string

	// Name of the test that this result represents.
	Name string

	// Duration of the test run.
	Duration time.Duration

	// Success flag.
	Success bool

	// Name of the location where the test was run.
	RunLocation string

	// Diagnostic message for the result.
	Message string
}

// Creates a new availability telemetry item with the specified test name,
// duration and success code.
func NewAvailabilityTelemetry(name string, duration time.Duration, success bool) *AvailabilityTelemetry {
	return &AvailabilityTelemetry{
		Name:     name,
		Duration: duration,
		Success:  success,
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

// Sets the timestamp and duration of this telemetry item based on the provided
// start and end times.
func (telem *AvailabilityTelemetry) MarkTime(startTime, endTime time.Time) {
	telem.Timestamp = startTime
	telem.Duration = endTime.Sub(startTime)
}

func (telem *AvailabilityTelemetry) TelemetryData() TelemetryData {
	data := contracts.NewAvailabilityData()
	data.Name = telem.Name
	data.Duration = formatDuration(telem.Duration)
	data.Success = telem.Success
	data.RunLocation = telem.RunLocation
	data.Message = telem.Message
	data.Properties = telem.Properties
	data.Id = telem.Id
	data.Measurements = telem.Measurements

	return data
}

// Page view telemetry items represent generic actions on a page like a button
// click.
type PageViewTelemetry struct {
	BaseTelemetry
	BaseTelemetryMeasurements

	// Request URL with all query string parameters
	Url string

	// Request duration.
	Duration time.Duration

	// Event name.
	Name string
}

// Creates a new page view telemetry item with the specified name and url.
func NewPageViewTelemetry(name, url string) *PageViewTelemetry {
	return &PageViewTelemetry{
		Name: name,
		Url:  url,
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

// Sets the timestamp and duration of this telemetry item based on the provided
// start and end times.
func (telem *PageViewTelemetry) MarkTime(startTime, endTime time.Time) {
	telem.Timestamp = startTime
	telem.Duration = endTime.Sub(startTime)
}

func (telem *PageViewTelemetry) TelemetryData() TelemetryData {
	data := contracts.NewPageViewData()
	data.Url = telem.Url
	data.Duration = formatDuration(telem.Duration)
	data.Name = telem.Name
	data.Properties = telem.Properties
	data.Measurements = telem.Measurements
	return data
}

func formatDuration(d time.Duration) string {
	ticks := int64(d/(time.Nanosecond*100)) % 10000000
	seconds := int64(d/time.Second) % 60
	minutes := int64(d/time.Minute) % 60
	hours := int64(d/time.Hour) % 24
	days := int64(d / (time.Hour * 24))

	return fmt.Sprintf("%d.%02d:%02d:%02d.%07d", days, hours, minutes, seconds, ticks)
}
