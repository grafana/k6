package contracts

// NOTE: This file was automatically generated.

// An instance of Exception represents a handled or unhandled exception that
// occurred during execution of the monitored application.
type ExceptionData struct {
	Domain

	// Schema version
	Ver int `json:"ver"`

	// Exception chain - list of inner exceptions.
	Exceptions []*ExceptionDetails `json:"exceptions"`

	// Severity level. Mostly used to indicate exception severity level when it is
	// reported by logging library.
	SeverityLevel SeverityLevel `json:"severityLevel"`

	// Identifier of where the exception was thrown in code. Used for exceptions
	// grouping. Typically a combination of exception type and a function from the
	// call stack.
	ProblemId string `json:"problemId"`

	// Collection of custom properties.
	Properties map[string]string `json:"properties,omitempty"`

	// Collection of custom measurements.
	Measurements map[string]float64 `json:"measurements,omitempty"`
}

// Returns the name used when this is embedded within an Envelope container.
func (data *ExceptionData) EnvelopeName(key string) string {
	if key != "" {
		return "Microsoft.ApplicationInsights." + key + ".Exception"
	} else {
		return "Microsoft.ApplicationInsights.Exception"
	}
}

// Returns the base type when placed within a Data object container.
func (data *ExceptionData) BaseType() string {
	return "ExceptionData"
}

// Truncates string fields that exceed their maximum supported sizes for this
// object and all objects it references.  Returns a warning for each affected
// field.
func (data *ExceptionData) Sanitize() []string {
	var warnings []string

	for _, ptr := range data.Exceptions {
		warnings = append(warnings, ptr.Sanitize()...)
	}

	if len(data.ProblemId) > 1024 {
		data.ProblemId = data.ProblemId[:1024]
		warnings = append(warnings, "ExceptionData.ProblemId exceeded maximum length of 1024")
	}

	if data.Properties != nil {
		for k, v := range data.Properties {
			if len(v) > 8192 {
				data.Properties[k] = v[:8192]
				warnings = append(warnings, "ExceptionData.Properties has value with length exceeding max of 8192: "+k)
			}
			if len(k) > 150 {
				data.Properties[k[:150]] = data.Properties[k]
				delete(data.Properties, k)
				warnings = append(warnings, "ExceptionData.Properties has key with length exceeding max of 150: "+k)
			}
		}
	}

	if data.Measurements != nil {
		for k, v := range data.Measurements {
			if len(k) > 150 {
				data.Measurements[k[:150]] = v
				delete(data.Measurements, k)
				warnings = append(warnings, "ExceptionData.Measurements has key with length exceeding max of 150: "+k)
			}
		}
	}

	return warnings
}

// Creates a new ExceptionData instance with default values set by the schema.
func NewExceptionData() *ExceptionData {
	return &ExceptionData{
		Ver: 2,
	}
}
