package contracts

// NOTE: This file was automatically generated.

// Exception details of the exception in a chain.
type ExceptionDetails struct {

	// In case exception is nested (outer exception contains inner one), the id
	// and outerId properties are used to represent the nesting.
	Id int `json:"id"`

	// The value of outerId is a reference to an element in ExceptionDetails that
	// represents the outer exception
	OuterId int `json:"outerId"`

	// Exception type name.
	TypeName string `json:"typeName"`

	// Exception message.
	Message string `json:"message"`

	// Indicates if full exception stack is provided in the exception. The stack
	// may be trimmed, such as in the case of a StackOverflow exception.
	HasFullStack bool `json:"hasFullStack"`

	// Text describing the stack. Either stack or parsedStack should have a value.
	Stack string `json:"stack"`

	// List of stack frames. Either stack or parsedStack should have a value.
	ParsedStack []*StackFrame `json:"parsedStack,omitempty"`
}

// Truncates string fields that exceed their maximum supported sizes for this
// object and all objects it references.  Returns a warning for each affected
// field.
func (data *ExceptionDetails) Sanitize() []string {
	var warnings []string

	if len(data.TypeName) > 1024 {
		data.TypeName = data.TypeName[:1024]
		warnings = append(warnings, "ExceptionDetails.TypeName exceeded maximum length of 1024")
	}

	if len(data.Message) > 32768 {
		data.Message = data.Message[:32768]
		warnings = append(warnings, "ExceptionDetails.Message exceeded maximum length of 32768")
	}

	if len(data.Stack) > 32768 {
		data.Stack = data.Stack[:32768]
		warnings = append(warnings, "ExceptionDetails.Stack exceeded maximum length of 32768")
	}

	for _, ptr := range data.ParsedStack {
		warnings = append(warnings, ptr.Sanitize()...)
	}

	return warnings
}

// Creates a new ExceptionDetails instance with default values set by the schema.
func NewExceptionDetails() *ExceptionDetails {
	return &ExceptionDetails{
		HasFullStack: true,
	}
}
