package kafka

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSerializeString tests the serialization of a string.
func TestSerializeString(t *testing.T) {
	stringSerde := &StringSerde{}
	expected := []byte("test")
	actual, err := stringSerde.Serialize("test", nil)
	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

// TestSerializeStringFails tests the serialization of a string
// and fails on invalid data type.
func TestSerializeStringFails(t *testing.T) {
	stringSerde := &StringSerde{}
	actual, err := stringSerde.Serialize(1, nil)
	assert.Nil(t, actual)
	assert.NotNil(t, err)
	assert.Equal(t, ErrInvalidDataType, err)
}

// TestDeserializeString tests the deserialization of a string.
func TestDeserializeString(t *testing.T) {
	stringSerde := &StringSerde{}
	expected := "test"
	actual, err := stringSerde.Deserialize([]byte("test"), nil)
	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}
