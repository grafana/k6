package contracts

// NOTE: This file was automatically generated.

// An instance of Request represents completion of an external request to the
// application to do work and contains a summary of that request execution and
// the results.
type RequestData struct {
	Domain

	// Schema version
	Ver int `json:"ver"`

	// Identifier of a request call instance. Used for correlation between request
	// and other telemetry items.
	Id string `json:"id"`

	// Source of the request. Examples are the instrumentation key of the caller
	// or the ip address of the caller.
	Source string `json:"source"`

	// Name of the request. Represents code path taken to process request. Low
	// cardinality value to allow better grouping of requests. For HTTP requests
	// it represents the HTTP method and URL path template like 'GET
	// /values/{id}'.
	Name string `json:"name"`

	// Request duration in format: DD.HH:MM:SS.MMMMMM. Must be less than 1000
	// days.
	Duration string `json:"duration"`

	// Result of a request execution. HTTP status code for HTTP requests.
	ResponseCode string `json:"responseCode"`

	// Indication of successfull or unsuccessfull call.
	Success bool `json:"success"`

	// Request URL with all query string parameters.
	Url string `json:"url"`

	// Collection of custom properties.
	Properties map[string]string `json:"properties,omitempty"`

	// Collection of custom measurements.
	Measurements map[string]float64 `json:"measurements,omitempty"`
}

// Returns the name used when this is embedded within an Envelope container.
func (data *RequestData) EnvelopeName(key string) string {
	if key != "" {
		return "Microsoft.ApplicationInsights." + key + ".Request"
	} else {
		return "Microsoft.ApplicationInsights.Request"
	}
}

// Returns the base type when placed within a Data object container.
func (data *RequestData) BaseType() string {
	return "RequestData"
}

// Truncates string fields that exceed their maximum supported sizes for this
// object and all objects it references.  Returns a warning for each affected
// field.
func (data *RequestData) Sanitize() []string {
	var warnings []string

	if len(data.Id) > 128 {
		data.Id = data.Id[:128]
		warnings = append(warnings, "RequestData.Id exceeded maximum length of 128")
	}

	if len(data.Source) > 1024 {
		data.Source = data.Source[:1024]
		warnings = append(warnings, "RequestData.Source exceeded maximum length of 1024")
	}

	if len(data.Name) > 1024 {
		data.Name = data.Name[:1024]
		warnings = append(warnings, "RequestData.Name exceeded maximum length of 1024")
	}

	if len(data.ResponseCode) > 1024 {
		data.ResponseCode = data.ResponseCode[:1024]
		warnings = append(warnings, "RequestData.ResponseCode exceeded maximum length of 1024")
	}

	if len(data.Url) > 2048 {
		data.Url = data.Url[:2048]
		warnings = append(warnings, "RequestData.Url exceeded maximum length of 2048")
	}

	if data.Properties != nil {
		for k, v := range data.Properties {
			if len(v) > 8192 {
				data.Properties[k] = v[:8192]
				warnings = append(warnings, "RequestData.Properties has value with length exceeding max of 8192: "+k)
			}
			if len(k) > 150 {
				data.Properties[k[:150]] = data.Properties[k]
				delete(data.Properties, k)
				warnings = append(warnings, "RequestData.Properties has key with length exceeding max of 150: "+k)
			}
		}
	}

	if data.Measurements != nil {
		for k, v := range data.Measurements {
			if len(k) > 150 {
				data.Measurements[k[:150]] = v
				delete(data.Measurements, k)
				warnings = append(warnings, "RequestData.Measurements has key with length exceeding max of 150: "+k)
			}
		}
	}

	return warnings
}

// Creates a new RequestData instance with default values set by the schema.
func NewRequestData() *RequestData {
	return &RequestData{
		Ver: 2,
	}
}
