package contracts

// NOTE: This file was automatically generated.

// An instance of Remote Dependency represents an interaction of the monitored
// component with a remote component/service like SQL or an HTTP endpoint.
type RemoteDependencyData struct {
	Domain

	// Schema version
	Ver int `json:"ver"`

	// Name of the command initiated with this dependency call. Low cardinality
	// value. Examples are stored procedure name and URL path template.
	Name string `json:"name"`

	// Identifier of a dependency call instance. Used for correlation with the
	// request telemetry item corresponding to this dependency call.
	Id string `json:"id"`

	// Result code of a dependency call. Examples are SQL error code and HTTP
	// status code.
	ResultCode string `json:"resultCode"`

	// Request duration in format: DD.HH:MM:SS.MMMMMM. Must be less than 1000
	// days.
	Duration string `json:"duration"`

	// Indication of successfull or unsuccessfull call.
	Success bool `json:"success"`

	// Command initiated by this dependency call. Examples are SQL statement and
	// HTTP URL's with all query parameters.
	Data string `json:"data"`

	// Target site of a dependency call. Examples are server name, host address.
	Target string `json:"target"`

	// Dependency type name. Very low cardinality value for logical grouping of
	// dependencies and interpretation of other fields like commandName and
	// resultCode. Examples are SQL, Azure table, and HTTP.
	Type string `json:"type"`

	// Collection of custom properties.
	Properties map[string]string `json:"properties,omitempty"`

	// Collection of custom measurements.
	Measurements map[string]float64 `json:"measurements,omitempty"`
}

// Returns the name used when this is embedded within an Envelope container.
func (data *RemoteDependencyData) EnvelopeName(key string) string {
	if key != "" {
		return "Microsoft.ApplicationInsights." + key + ".RemoteDependency"
	} else {
		return "Microsoft.ApplicationInsights.RemoteDependency"
	}
}

// Returns the base type when placed within a Data object container.
func (data *RemoteDependencyData) BaseType() string {
	return "RemoteDependencyData"
}

// Truncates string fields that exceed their maximum supported sizes for this
// object and all objects it references.  Returns a warning for each affected
// field.
func (data *RemoteDependencyData) Sanitize() []string {
	var warnings []string

	if len(data.Name) > 1024 {
		data.Name = data.Name[:1024]
		warnings = append(warnings, "RemoteDependencyData.Name exceeded maximum length of 1024")
	}

	if len(data.Id) > 128 {
		data.Id = data.Id[:128]
		warnings = append(warnings, "RemoteDependencyData.Id exceeded maximum length of 128")
	}

	if len(data.ResultCode) > 1024 {
		data.ResultCode = data.ResultCode[:1024]
		warnings = append(warnings, "RemoteDependencyData.ResultCode exceeded maximum length of 1024")
	}

	if len(data.Data) > 8192 {
		data.Data = data.Data[:8192]
		warnings = append(warnings, "RemoteDependencyData.Data exceeded maximum length of 8192")
	}

	if len(data.Target) > 1024 {
		data.Target = data.Target[:1024]
		warnings = append(warnings, "RemoteDependencyData.Target exceeded maximum length of 1024")
	}

	if len(data.Type) > 1024 {
		data.Type = data.Type[:1024]
		warnings = append(warnings, "RemoteDependencyData.Type exceeded maximum length of 1024")
	}

	if data.Properties != nil {
		for k, v := range data.Properties {
			if len(v) > 8192 {
				data.Properties[k] = v[:8192]
				warnings = append(warnings, "RemoteDependencyData.Properties has value with length exceeding max of 8192: "+k)
			}
			if len(k) > 150 {
				data.Properties[k[:150]] = data.Properties[k]
				delete(data.Properties, k)
				warnings = append(warnings, "RemoteDependencyData.Properties has key with length exceeding max of 150: "+k)
			}
		}
	}

	if data.Measurements != nil {
		for k, v := range data.Measurements {
			if len(k) > 150 {
				data.Measurements[k[:150]] = v
				delete(data.Measurements, k)
				warnings = append(warnings, "RemoteDependencyData.Measurements has key with length exceeding max of 150: "+k)
			}
		}
	}

	return warnings
}

// Creates a new RemoteDependencyData instance with default values set by the schema.
func NewRemoteDependencyData() *RemoteDependencyData {
	return &RemoteDependencyData{
		Ver:     2,
		Success: true,
	}
}
