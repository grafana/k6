package kafka

type StringSerde struct {
	Serdes
}

const (
	String SchemaType = "STRING"
)

// Serialize serializes a string to bytes.
func (*StringSerde) Serialize(data any, _ *Schema) ([]byte, *Xk6KafkaError) {
	switch data := data.(type) {
	case string:
		return []byte(data), nil
	default:
		return nil, ErrInvalidDataType
	}
}

// Deserialize deserializes a string from bytes.
func (*StringSerde) Deserialize(data []byte, _ *Schema) (any, *Xk6KafkaError) {
	return string(data), nil
}
