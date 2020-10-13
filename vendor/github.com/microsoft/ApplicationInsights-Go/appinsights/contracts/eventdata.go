package contracts

// NOTE: This file was automatically generated.

// Instances of Event represent structured event records that can be grouped
// and searched by their properties. Event data item also creates a metric of
// event count by name.
type EventData struct {
	Domain

	// Schema version
	Ver int `json:"ver"`

	// Event name. Keep it low cardinality to allow proper grouping and useful
	// metrics.
	Name string `json:"name"`

	// Collection of custom properties.
	Properties map[string]string `json:"properties,omitempty"`

	// Collection of custom measurements.
	Measurements map[string]float64 `json:"measurements,omitempty"`
}

// Returns the name used when this is embedded within an Envelope container.
func (data *EventData) EnvelopeName(key string) string {
	if key != "" {
		return "Microsoft.ApplicationInsights." + key + ".Event"
	} else {
		return "Microsoft.ApplicationInsights.Event"
	}
}

// Returns the base type when placed within a Data object container.
func (data *EventData) BaseType() string {
	return "EventData"
}

// Truncates string fields that exceed their maximum supported sizes for this
// object and all objects it references.  Returns a warning for each affected
// field.
func (data *EventData) Sanitize() []string {
	var warnings []string

	if len(data.Name) > 512 {
		data.Name = data.Name[:512]
		warnings = append(warnings, "EventData.Name exceeded maximum length of 512")
	}

	if data.Properties != nil {
		for k, v := range data.Properties {
			if len(v) > 8192 {
				data.Properties[k] = v[:8192]
				warnings = append(warnings, "EventData.Properties has value with length exceeding max of 8192: "+k)
			}
			if len(k) > 150 {
				data.Properties[k[:150]] = data.Properties[k]
				delete(data.Properties, k)
				warnings = append(warnings, "EventData.Properties has key with length exceeding max of 150: "+k)
			}
		}
	}

	if data.Measurements != nil {
		for k, v := range data.Measurements {
			if len(k) > 150 {
				data.Measurements[k[:150]] = v
				delete(data.Measurements, k)
				warnings = append(warnings, "EventData.Measurements has key with length exceeding max of 150: "+k)
			}
		}
	}

	return warnings
}

// Creates a new EventData instance with default values set by the schema.
func NewEventData() *EventData {
	return &EventData{
		Ver: 2,
	}
}
