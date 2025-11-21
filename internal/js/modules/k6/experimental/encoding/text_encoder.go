package encoding

import (
	"errors"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
)

// TextEncoder represents an encoder that will generate a byte stream
// with UTF-8 encoding.
type TextEncoder struct {
	// Encoding always holds the `utf-8` value.
	// FIXME: this should be TextEncoder.prototype.encoding instead
	Encoding Name

	encoder encoding.Encoding
}

// NewTextEncoder returns a new TextEncoder object instance that will
// generate a byte stream with UTF-8 encoding.
func NewTextEncoder() *TextEncoder {
	return &TextEncoder{
		encoder:  unicode.UTF8,
		Encoding: UTF8EncodingFormat,
	}
}

// Encode takes a string as input and returns an encoded byte stream.
func (te *TextEncoder) Encode(text string) ([]byte, error) {
	if te.encoder == nil {
		return nil, errors.New("encoding not set")
	}

	enc := te.encoder.NewEncoder()
	encoded, err := enc.String(text)
	if err != nil {
		return nil, NewError(TypeError, "unable to encode text; reason: "+err.Error())
	}

	return []byte(encoded), nil
}
