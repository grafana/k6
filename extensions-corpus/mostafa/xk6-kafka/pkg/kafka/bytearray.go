package kafka

type ByteArraySerde struct {
	Serdes
}

const (
	Bytes SchemaType = "BYTES"
)

// Serialize serializes the given data into a byte array.
func (*ByteArraySerde) Serialize(data any, _ *Schema) ([]byte, *Xk6KafkaError) {
	switch data := data.(type) {
	case []byte:
		return data, nil
	case []any:
		arr := make([]byte, len(data))
		for i, u := range data {
			if u, ok := u.(float64); ok {
				arr[i] = byte(u)
			} else {
				return nil, ErrFailedTypeCast
			}
		}
		return arr, nil
	default:
		return nil, ErrInvalidDataType
	}
}

// Deserialize returns the data as-is, because it is already a byte array.
func (*ByteArraySerde) Deserialize(data []byte, _ *Schema) (any, *Xk6KafkaError) {
	return data, nil
}
