package contracts

// NOTE: This file was automatically generated.

// System variables for a telemetry item.
type Envelope struct {

	// Envelope version. For internal use only. By assigning this the default, it
	// will not be serialized within the payload unless changed to a value other
	// than #1.
	Ver int `json:"ver"`

	// Type name of telemetry data item.
	Name string `json:"name"`

	// Event date time when telemetry item was created. This is the wall clock
	// time on the client when the event was generated. There is no guarantee that
	// the client's time is accurate. This field must be formatted in UTC ISO 8601
	// format, with a trailing 'Z' character, as described publicly on
	// https://en.wikipedia.org/wiki/ISO_8601#UTC. Note: the number of decimal
	// seconds digits provided are variable (and unspecified). Consumers should
	// handle this, i.e. managed code consumers should not use format 'O' for
	// parsing as it specifies a fixed length. Example:
	// 2009-06-15T13:45:30.0000000Z.
	Time string `json:"time"`

	// Sampling rate used in application. This telemetry item represents 1 /
	// sampleRate actual telemetry items.
	SampleRate float64 `json:"sampleRate"`

	// Sequence field used to track absolute order of uploaded events.
	Seq string `json:"seq"`

	// The application's instrumentation key. The key is typically represented as
	// a GUID, but there are cases when it is not a guid. No code should rely on
	// iKey being a GUID. Instrumentation key is case insensitive.
	IKey string `json:"iKey"`

	// Key/value collection of context properties. See ContextTagKeys for
	// information on available properties.
	Tags map[string]string `json:"tags,omitempty"`

	// Telemetry data item.
	Data interface{} `json:"data"`
}

// Truncates string fields that exceed their maximum supported sizes for this
// object and all objects it references.  Returns a warning for each affected
// field.
func (data *Envelope) Sanitize() []string {
	var warnings []string

	if len(data.Name) > 1024 {
		data.Name = data.Name[:1024]
		warnings = append(warnings, "Envelope.Name exceeded maximum length of 1024")
	}

	if len(data.Time) > 64 {
		data.Time = data.Time[:64]
		warnings = append(warnings, "Envelope.Time exceeded maximum length of 64")
	}

	if len(data.Seq) > 64 {
		data.Seq = data.Seq[:64]
		warnings = append(warnings, "Envelope.Seq exceeded maximum length of 64")
	}

	if len(data.IKey) > 40 {
		data.IKey = data.IKey[:40]
		warnings = append(warnings, "Envelope.IKey exceeded maximum length of 40")
	}

	return warnings
}

// Creates a new Envelope instance with default values set by the schema.
func NewEnvelope() *Envelope {
	return &Envelope{
		Ver:        1,
		SampleRate: 100.0,
	}
}
