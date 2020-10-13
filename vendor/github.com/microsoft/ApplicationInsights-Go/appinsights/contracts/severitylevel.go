package contracts

// NOTE: This file was automatically generated.

// Defines the level of severity for the event.
type SeverityLevel int

const (
	Verbose     SeverityLevel = 0
	Information SeverityLevel = 1
	Warning     SeverityLevel = 2
	Error       SeverityLevel = 3
	Critical    SeverityLevel = 4
)

func (value SeverityLevel) String() string {
	switch int(value) {
	case 0:
		return "Verbose"
	case 1:
		return "Information"
	case 2:
		return "Warning"
	case 3:
		return "Error"
	case 4:
		return "Critical"
	default:
		return "<unknown SeverityLevel>"
	}
}
