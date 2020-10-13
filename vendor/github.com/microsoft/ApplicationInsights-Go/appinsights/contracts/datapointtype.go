package contracts

// NOTE: This file was automatically generated.

// Type of the metric data measurement.
type DataPointType int

const (
	Measurement DataPointType = 0
	Aggregation DataPointType = 1
)

func (value DataPointType) String() string {
	switch int(value) {
	case 0:
		return "Measurement"
	case 1:
		return "Aggregation"
	default:
		return "<unknown DataPointType>"
	}
}
