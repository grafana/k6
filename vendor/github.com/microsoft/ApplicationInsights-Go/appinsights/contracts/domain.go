package contracts

// NOTE: This file was automatically generated.

// The abstract common base of all domains.
type Domain struct {
}

// Truncates string fields that exceed their maximum supported sizes for this
// object and all objects it references.  Returns a warning for each affected
// field.
func (data *Domain) Sanitize() []string {
	var warnings []string

	return warnings
}

// Creates a new Domain instance with default values set by the schema.
func NewDomain() *Domain {
	return &Domain{}
}
