package contracts

// NOTE: This file was automatically generated.

// Instances of Message represent printf-like trace statements that are
// text-searched. Log4Net, NLog and other text-based log file entries are
// translated into intances of this type. The message does not have
// measurements.
type MessageData struct {
	Domain

	// Schema version
	Ver int `json:"ver"`

	// Trace message
	Message string `json:"message"`

	// Trace severity level.
	SeverityLevel SeverityLevel `json:"severityLevel"`

	// Collection of custom properties.
	Properties map[string]string `json:"properties,omitempty"`
}

// Returns the name used when this is embedded within an Envelope container.
func (data *MessageData) EnvelopeName(key string) string {
	if key != "" {
		return "Microsoft.ApplicationInsights." + key + ".Message"
	} else {
		return "Microsoft.ApplicationInsights.Message"
	}
}

// Returns the base type when placed within a Data object container.
func (data *MessageData) BaseType() string {
	return "MessageData"
}

// Truncates string fields that exceed their maximum supported sizes for this
// object and all objects it references.  Returns a warning for each affected
// field.
func (data *MessageData) Sanitize() []string {
	var warnings []string

	if len(data.Message) > 32768 {
		data.Message = data.Message[:32768]
		warnings = append(warnings, "MessageData.Message exceeded maximum length of 32768")
	}

	if data.Properties != nil {
		for k, v := range data.Properties {
			if len(v) > 8192 {
				data.Properties[k] = v[:8192]
				warnings = append(warnings, "MessageData.Properties has value with length exceeding max of 8192: "+k)
			}
			if len(k) > 150 {
				data.Properties[k[:150]] = data.Properties[k]
				delete(data.Properties, k)
				warnings = append(warnings, "MessageData.Properties has key with length exceeding max of 150: "+k)
			}
		}
	}

	return warnings
}

// Creates a new MessageData instance with default values set by the schema.
func NewMessageData() *MessageData {
	return &MessageData{
		Ver: 2,
	}
}
