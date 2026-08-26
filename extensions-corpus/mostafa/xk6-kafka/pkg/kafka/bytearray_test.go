package kafka

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSerializeByteArray tests the serialization of a byte array.
func TestSerializeByteArray(t *testing.T) {
	byteArraySerde := &ByteArraySerde{}
	expected := []byte{1, 2, 3}
	actual, err := byteArraySerde.Serialize([]byte{1, 2, 3}, nil)
	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

// TestSerializeInterfaceArray tests the serialization of an interface array.
func TestSerializeInterfaceArray(t *testing.T) {
	byteArraySerde := &ByteArraySerde{}
	expected := []byte{0x41, 0x42, 0x43}
	actual, err := byteArraySerde.Serialize([]any{65.0, 66.0, 67.0}, nil)
	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

// TestSerializeInterfaceArrayFails tests the serialization of an interface array
// and fails on mixed data type.
func TestSerializeInterfaceArrayFails(t *testing.T) {
	byteArraySerde := &ByteArraySerde{}
	actual, err := byteArraySerde.Serialize([]any{65.0, 66.0, "a"}, nil)
	assert.Nil(t, actual)
	assert.NotNil(t, err)
	assert.Equal(t, ErrFailedTypeCast, err)
}

// TestSerializeByteArrayFails tests the serialization of a byte array
// and fails on invalid data type.
func TestSerializeByteArrayFails(t *testing.T) {
	byteArraySerde := &ByteArraySerde{}
	actual, err := byteArraySerde.Serialize(1, nil)
	assert.Nil(t, actual)
	assert.NotNil(t, err)
	assert.Equal(t, ErrInvalidDataType, err)
}

// TestDeserializeByteArray tests the deserialization of a byte array.
func TestDeserializeByteArray(t *testing.T) {
	byteArraySerde := &ByteArraySerde{}
	expected := []byte{1, 2, 3}
	actual, err := byteArraySerde.Deserialize([]byte{1, 2, 3}, nil)
	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}
