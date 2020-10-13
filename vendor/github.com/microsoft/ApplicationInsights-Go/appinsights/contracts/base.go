package contracts

// NOTE: This file was automatically generated.

// Data struct to contain only C section with custom fields.
type Base struct {

	// Name of item (B section) if any. If telemetry data is derived straight from
	// this, this should be null.
	BaseType string `json:"baseType"`
}

// Truncates string fields that exceed their maximum supported sizes for this
// object and all objects it references.  Returns a warning for each affected
// field.
func (data *Base) Sanitize() []string {
	var warnings []string

	return warnings
}

// Creates a new Base instance with default values set by the schema.
func NewBase() *Base {
	return &Base{}
}
