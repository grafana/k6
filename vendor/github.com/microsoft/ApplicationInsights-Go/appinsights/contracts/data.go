package contracts

// NOTE: This file was automatically generated.

// Data struct to contain both B and C sections.
type Data struct {
	Base

	// Container for data item (B section).
	BaseData interface{} `json:"baseData"`
}

// Truncates string fields that exceed their maximum supported sizes for this
// object and all objects it references.  Returns a warning for each affected
// field.
func (data *Data) Sanitize() []string {
	var warnings []string

	return warnings
}

// Creates a new Data instance with default values set by the schema.
func NewData() *Data {
	return &Data{}
}
