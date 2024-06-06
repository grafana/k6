package webcrypto

import (
	"fmt"

	"github.com/grafana/sobek"
)

// bitLength is a type alias for the length of a bits collection.
type bitLength int

// asByteLength returns the length of the bits collection in bytes.
func (b bitLength) asByteLength() byteLength {
	return byteLength(b) / 8
}

// byteLength is a type alias for the length of a byte slice.
type byteLength int

// asBitLength returns the length of the byte slice in bits.
func (b byteLength) asBitLength() bitLength {
	return bitLength(b) * 8
}

// ToBytes tries to return a byte slice from compatible types.
func ToBytes(data interface{}) ([]byte, error) {
	switch dt := data.(type) {
	case []byte:
		return dt, nil
	case string:
		return []byte(dt), nil
	case sobek.ArrayBuffer:
		return dt.Bytes(), nil
	default:
		return nil, fmt.Errorf("invalid type %T, expected string, []byte or ArrayBuffer", data)
	}
}
