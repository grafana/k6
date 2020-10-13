package contracts

// NOTE: This file was automatically generated.

// An instance of the Metric item is a list of measurements (single data
// points) and/or aggregations.
type MetricData struct {
	Domain

	// Schema version
	Ver int `json:"ver"`

	// List of metrics. Only one metric in the list is currently supported by
	// Application Insights storage. If multiple data points were sent only the
	// first one will be used.
	Metrics []*DataPoint `json:"metrics"`

	// Collection of custom properties.
	Properties map[string]string `json:"properties,omitempty"`
}

// Returns the name used when this is embedded within an Envelope container.
func (data *MetricData) EnvelopeName(key string) string {
	if key != "" {
		return "Microsoft.ApplicationInsights." + key + ".Metric"
	} else {
		return "Microsoft.ApplicationInsights.Metric"
	}
}

// Returns the base type when placed within a Data object container.
func (data *MetricData) BaseType() string {
	return "MetricData"
}

// Truncates string fields that exceed their maximum supported sizes for this
// object and all objects it references.  Returns a warning for each affected
// field.
func (data *MetricData) Sanitize() []string {
	var warnings []string

	for _, ptr := range data.Metrics {
		warnings = append(warnings, ptr.Sanitize()...)
	}

	if data.Properties != nil {
		for k, v := range data.Properties {
			if len(v) > 8192 {
				data.Properties[k] = v[:8192]
				warnings = append(warnings, "MetricData.Properties has value with length exceeding max of 8192: "+k)
			}
			if len(k) > 150 {
				data.Properties[k[:150]] = data.Properties[k]
				delete(data.Properties, k)
				warnings = append(warnings, "MetricData.Properties has key with length exceeding max of 150: "+k)
			}
		}
	}

	return warnings
}

// Creates a new MetricData instance with default values set by the schema.
func NewMetricData() *MetricData {
	return &MetricData{
		Ver: 2,
	}
}
