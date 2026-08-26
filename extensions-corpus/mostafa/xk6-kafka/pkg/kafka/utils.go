package kafka

import (
	"encoding/base64"
	"encoding/json"

	"github.com/grafana/sobek"
)

// freeze disallows resetting or changing the properties of the object.
func freeze(o *sobek.Object) *Xk6KafkaError {
	if o == nil {
		return NewXk6KafkaError(
			failedFreezeObject,
			"Failed to freeze JS object.",
			errObjectMustNotBeNil,
		)
	}

	for _, key := range o.Keys() {
		if err := o.DefineDataProperty(
			key, o.Get(key), sobek.FLAG_FALSE, sobek.FLAG_FALSE, sobek.FLAG_TRUE); err != nil {
			return NewXk6KafkaError(failedFreezeObject, "Failed to freeze JS object.", err)
		}
	}

	return nil
}

// base64ToBytes converts the base64 encoded data to bytes.
func base64ToBytes(data string) ([]byte, *Xk6KafkaError) {
	result, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, NewXk6KafkaError(failedDecodeBase64, "Failed to decode base64 string", err)
	}
	return result, nil
}

// isBase64Encoded checks whether the data is base64 encoded.
func isBase64Encoded(data string) bool {
	_, err := base64ToBytes(data)
	return err == nil
}

// toJSONBytes encodes a map into JSON bytes.
func toJSONBytes(data any) ([]byte, *Xk6KafkaError) {
	if data, ok := data.(map[string]any); ok {
		if jsonData, err := json.Marshal(data); err == nil {
			return jsonData, nil
		}
	}

	return nil, ErrInvalidDataType
}

func toMap(data []byte) (map[string]any, *Xk6KafkaError) {
	var js map[string]any
	if err := json.Unmarshal(data, &js); err != nil {
		return nil, NewXk6KafkaError(failedUnmarshalJSON, "Failed to decode data", err)
	}
	return js, nil
}

func isJSON(data []byte) bool {
	if json.Valid(data) {
		if _, err := toMap(data); err != nil {
			return false
		}
		return true
	}
	return false
}
