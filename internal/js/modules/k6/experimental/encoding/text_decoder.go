package encoding

import (
	"errors"
	"fmt"
	"strings"

	"github.com/grafana/sobek"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// TextDecoder represents a decoder for a specific text encoding, such
// as UTF-8, UTF-16, ISO-8859-2, etc.
//
// A decoder takes a stream of bytes as input and emits a stream of code points.
type TextDecoder struct {
	TextDecoderCommon

	decoder   encoding.Encoding
	transform transform.Transformer

	rt *sobek.Runtime

	buffer []byte
}

// TextDecoderOptions represents the options that can be passed to the
// `TextDecoder` constructor.
type TextDecoderOptions struct {
	// Fatal holds a boolean value indicating if
	// the `TextDecoder.decode()`` method must throw
	// a `TypeError` when decoding invalid data.
	//
	// It defaults to `false`, which means that the
	// decoder will substitute malformed data with a
	// replacement character.
	Fatal bool `json:"fatal"`

	// IgnoreBOM holds a boolean value indicating
	// whether the byte order mark is ignored.
	IgnoreBOM bool `json:"ignoreBOM"`
}

// TextDecoderCommon represents the common subset of the TextDecoder interface
// that is shared between the TextDecoder and TextDecoderStream interfaces.
type TextDecoderCommon struct {
	// Encoding holds the name of the decoder which is a string describing
	// the method the `TextDecoder` will use.
	Encoding Name `json:"encoding"`

	// Fatal holds a boolean value indicating if
	// the `TextDecoder.decode()`` method must throw
	// a `TypeError` when decoding invalid data.
	//
	// It defaults to `false`, which means that the
	// decoder will substitute malformed data with a
	// replacement character.
	Fatal bool `json:"fatal"`

	// IgnoreBOM holds a boolean value indicating
	// whether the byte order mark is ignored.
	IgnoreBOM bool `json:"ignoreBOM"`

	errorMode ErrorMode
}

// NewTextDecoder returns a new TextDecoder object instance that will
// generate a string from a byte stream with a specific encoding.
func NewTextDecoder(rt *sobek.Runtime, label string, options TextDecoderOptions) (*TextDecoder, error) {
	// Pick the encoding BOM policy accordingly
	bomPolicy := unicode.IgnoreBOM
	if !options.IgnoreBOM {
		bomPolicy = unicode.UseBOM
	}

	// 1.
	var enc Name
	var decoder encoding.Encoding
	switch strings.TrimSpace(strings.ToLower(label)) {
	case "",
		"unicode-1-1-utf-8",
		"unicode11utf8",
		"unicode20utf8",
		"utf-8",
		"utf8",
		"x-unicode20utf8":
		enc = UTF8EncodingFormat
		decoder = unicode.UTF8
	case "csunicode",
		"iso-10646-ucs-2",
		"ucs-2",
		"unicode",
		"unicodefeff",
		"utf-16",
		"utf-16le":
		enc = UTF16LEEncodingFormat
		decoder = unicode.UTF16(unicode.LittleEndian, bomPolicy)
	case "unicodefffe", "utf-16be":
		enc = UTF16BEEncodingFormat
		decoder = unicode.UTF16(unicode.BigEndian, bomPolicy)
	default:
		// 2.
		return nil, NewError(RangeError, fmt.Sprintf("unsupported enc: %s", label))
	}

	// 3.
	td := &TextDecoder{
		rt: rt,
	}
	td.Encoding = enc
	td.decoder = decoder

	// 4.
	if options.Fatal {
		td.Fatal = true
		td.errorMode = FatalErrorMode
	}

	// 5.
	td.IgnoreBOM = options.IgnoreBOM

	return td, nil
}

// replacementCharUTF8 is the UTF-8 encoded form of U+FFFD.
var replacementCharUTF8 = []byte{0xEF, 0xBF, 0xBD}

// Decode takes a byte stream as input and returns a string.
func (td *TextDecoder) Decode(buffer []byte, options TextDecodeOptions) (string, error) {
	if td.decoder == nil {
		return "", errors.New("encoding not set")
	}

	if td.Encoding == UTF8EncodingFormat {
		return td.decodeUTF8(buffer, options)
	}

	return td.decodeUTF16(buffer, options)
}

func (td *TextDecoder) decodeUTF8(buffer []byte, options TextDecodeOptions) (string, error) {
	stream := options.Stream

	if td.transform == nil {
		decoder := td.decoder.NewDecoder()
		if !td.IgnoreBOM {
			td.transform = unicode.BOMOverride(decoder)
		} else {
			td.transform = decoder
		}
	}

	combined := append(td.buffer, buffer...)
	td.buffer = td.buffer[:0]

	processed, leftover, hadInvalid := sanitizeUTF8Bytes(combined, stream)
	if len(leftover) > 0 {
		td.buffer = append(td.buffer, leftover...)
	}

	atEOF := !stream
	if stream && len(td.buffer) == 0 {
		atEOF = true
	}

	decoded, _, err := td.applyTransform(processed, atEOF)
	if err != nil && !errors.Is(err, transform.ErrShortDst) && !errors.Is(err, transform.ErrShortSrc) {
		if td.Fatal {
			td.resetState()
			return "", NewError(TypeError, "unable to decode text; reason: "+err.Error())
		}
		return "", err
	}

	if td.Fatal && hadInvalid {
		td.resetState()
		return "", NewError(TypeError, "invalid byte sequence")
	}

	if !stream {
		td.resetState()
	}

	return decoded, nil
}

func (td *TextDecoder) decodeUTF16(buffer []byte, options TextDecodeOptions) (string, error) {
	stream := options.Stream

	if td.transform == nil {
		td.transform = td.decoder.NewDecoder()
	}

	// Keep td.buffer as the canonical byte queue so sequences that span multiple
	// calls (e.g., surrogate pairs) remain intact until the transformer consumes
	// them.
	td.buffer = append(td.buffer, buffer...)

	processLen := len(td.buffer)
	var pending []byte

	if stream {
		if processLen%2 == 1 {
			processLen--
		}
	} else if processLen%2 == 1 {
		pending = append(pending, td.buffer[processLen-1])
		processLen--
	}

	var decoded string
	var err error
	consumed := 0

	if processLen > 0 {
		toDecode := td.buffer[:processLen]
		decoded, consumed, err = td.applyTransform(toDecode, !stream)
		td.buffer = append(td.buffer[:0], td.buffer[consumed:]...)
	} else {
		decoded, _, err = td.applyTransform(nil, !stream)
	}
	if err != nil && !errors.Is(err, transform.ErrShortDst) && !errors.Is(err, transform.ErrShortSrc) {
		if td.Fatal {
			td.resetState()
			return "", NewError(TypeError, "unable to decode text; reason: "+err.Error())
		}
		return "", err
	}

	var builder strings.Builder
	builder.WriteString(decoded)

	hadInvalid := false
	if len(pending) > 0 {
		hadInvalid = true
		if td.Fatal {
			td.resetState()
			return "", NewError(TypeError, "invalid byte sequence")
		}
		builder.WriteRune('\uFFFD')
	}

	result := builder.String()

	if td.Fatal {
		for _, r := range result {
			if r == '\uFFFD' {
				td.resetState()
				return "", NewError(TypeError, "invalid byte sequence")
			}
		}
	}

	if !stream {
		td.resetState()
	}

	if hadInvalid {
		return result, nil
	}

	return result, nil
}

func (td *TextDecoder) applyTransform(input []byte, atEOF bool) (string, int, error) {
	if td.transform == nil {
		return "", 0, errors.New("transformer not initialized")
	}

	if len(input) == 0 {
		if atEOF {
			empty := make([]byte, 0)
			if _, _, err := td.transform.Transform(empty, empty, true); err != nil &&
				!errors.Is(err, transform.ErrShortDst) &&
				!errors.Is(err, transform.ErrShortSrc) {
				return "", 0, err
			}
		}
		return "", 0, nil
	}

	destSize := len(input)*4 + 64
	if destSize < 64 {
		destSize = 64
	}
	dest := make([]byte, destSize)

	var (
		builder  strings.Builder
		src      = input
		consumed int
	)

	for len(src) > 0 {
		nDst, nSrc, err := td.transform.Transform(dest, src, atEOF)
		builder.Write(dest[:nDst])
		src = src[nSrc:]
		consumed += nSrc

		if err == nil {
			continue
		}

		if errors.Is(err, transform.ErrShortDst) {
			if nSrc == 0 {
				dest = make([]byte, len(dest)*2)
			}
			continue
		}

		if errors.Is(err, transform.ErrShortSrc) && !atEOF {
			break
		}

		if !errors.Is(err, transform.ErrShortSrc) && !errors.Is(err, transform.ErrShortDst) {
			return "", consumed, err
		}
	}

	return builder.String(), consumed, nil
}

func (td *TextDecoder) resetState() {
	if td.transform != nil {
		td.transform.Reset()
		td.transform = nil
	}
	td.buffer = nil
}

func sanitizeUTF8Bytes(data []byte, stream bool) (processed []byte, leftover []byte, hadInvalid bool) {
	if len(data) == 0 {
		return nil, nil, false
	}

	processed = make([]byte, 0, len(data))

	for i := 0; i < len(data); {
		b := data[i]

		if b < 0x80 {
			processed = append(processed, b)
			i++
			continue
		}

		var needed int
		switch {
		case b >= 0xC2 && b <= 0xDF:
			needed = 2
		case b >= 0xE0 && b <= 0xEF:
			needed = 3
		case b >= 0xF0 && b <= 0xF4:
			needed = 4
		default:
			hadInvalid = true
			processed = append(processed, replacementCharUTF8...)
			i++
			continue
		}

		validCount := 0
		incomplete := false
		for j := 1; j < needed; j++ {
			idx := i + j
			if idx >= len(data) {
				if stream {
					leftover = append([]byte{}, data[i:]...)
					return processed, leftover, hadInvalid
				}
				incomplete = true
				break
			}

			cb := data[idx]
			if !isValidContinuationByte(cb) || !passesUTF8RangeChecks(b, j, cb) {
				hadInvalid = true
				processed = append(processed, replacementCharUTF8...)
				i += 1 + validCount
				goto nextSequence
			}
			validCount++
		}

		if incomplete {
			hadInvalid = true
			processed = append(processed, replacementCharUTF8...)
			i += 1 + validCount
			continue
		}

		processed = append(processed, data[i:i+needed]...)
		i += needed
		continue

	nextSequence:
		continue
	}

	return processed, leftover, hadInvalid
}

func isValidContinuationByte(b byte) bool {
	return b&0xC0 == 0x80
}

func passesUTF8RangeChecks(start byte, position int, cont byte) bool {
	switch start {
	case 0xE0:
		if position == 1 {
			return cont >= 0xA0 && cont <= 0xBF
		}
	case 0xED:
		if position == 1 {
			return cont >= 0x80 && cont <= 0x9F
		}
	case 0xF0:
		if position == 1 {
			return cont >= 0x90 && cont <= 0xBF
		}
	case 0xF4:
		if position == 1 {
			return cont >= 0x80 && cont <= 0x8F
		}
	}
	return true
}

// GetBufferForDebug returns the internal buffer for debugging purposes
func (td *TextDecoder) GetBufferForDebug() []byte {
	return td.buffer
}

// TextDecodeOptions represents the options that can be passed to the
// TextDecoder.decode() method.
type TextDecodeOptions struct {
	// A boolean flag indicating whether additional data
	// will follow in subsequent calls to decode().
	//
	// Set to true if processing the data in chunks, and
	// false for the final chunk or if the data is not chunked.
	Stream bool `json:"stream"`
}

// Name is a type alias for the name of an encoding.
//
//nolint:revive
type Name = string

const (
	// UTF8EncodingFormat is the encoding format for utf-8
	UTF8EncodingFormat = "utf-8"

	// UTF16LEEncodingFormat is the encoding format for utf-16le
	UTF16LEEncodingFormat = "utf-16le"

	// UTF16BEEncodingFormat is the encoding format for utf-16be
	UTF16BEEncodingFormat = "utf-16be"
)

// ErrorMode is a type alias for the error mode of a TextDecoder.
type ErrorMode = string

const (
	// FatalErrorMode is the error mode for throwing a
	// TypeError when an invalid character is encountered.
	FatalErrorMode ErrorMode = "fatal"
)
