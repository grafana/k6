package contracts

// NOTE: This file was automatically generated.

// Stack frame information.
type StackFrame struct {

	// Level in the call stack. For the long stacks SDK may not report every
	// function in a call stack.
	Level int `json:"level"`

	// Method name.
	Method string `json:"method"`

	// Name of the assembly (dll, jar, etc.) containing this function.
	Assembly string `json:"assembly"`

	// File name or URL of the method implementation.
	FileName string `json:"fileName"`

	// Line number of the code implementation.
	Line int `json:"line"`
}

// Truncates string fields that exceed their maximum supported sizes for this
// object and all objects it references.  Returns a warning for each affected
// field.
func (data *StackFrame) Sanitize() []string {
	var warnings []string

	if len(data.Method) > 1024 {
		data.Method = data.Method[:1024]
		warnings = append(warnings, "StackFrame.Method exceeded maximum length of 1024")
	}

	if len(data.Assembly) > 1024 {
		data.Assembly = data.Assembly[:1024]
		warnings = append(warnings, "StackFrame.Assembly exceeded maximum length of 1024")
	}

	if len(data.FileName) > 1024 {
		data.FileName = data.FileName[:1024]
		warnings = append(warnings, "StackFrame.FileName exceeded maximum length of 1024")
	}

	return warnings
}

// Creates a new StackFrame instance with default values set by the schema.
func NewStackFrame() *StackFrame {
	return &StackFrame{}
}
