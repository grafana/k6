package base64

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"sync"
)

type Decoder interface {
	Decode([]byte) ([]byte, error)
}

type Encoder interface {
	Encode([]byte, []byte)
	EncodedLen(int) int
	EncodeToString([]byte) string
}

var muEncoder sync.RWMutex
var encoder Encoder = base64.RawURLEncoding
var muDecoder sync.RWMutex
var decoder Decoder = defaultDecoder{}

func SetEncoder(enc Encoder) {
	muEncoder.Lock()
	defer muEncoder.Unlock()
	encoder = enc
}

func getEncoder() Encoder {
	muEncoder.RLock()
	defer muEncoder.RUnlock()
	return encoder
}

func SetDecoder(dec Decoder) {
	muDecoder.Lock()
	defer muDecoder.Unlock()
	decoder = dec
}

func getDecoder() Decoder {
	muDecoder.RLock()
	defer muDecoder.RUnlock()
	return decoder
}

func Encode(src []byte) []byte {
	encoder := getEncoder()
	dst := make([]byte, encoder.EncodedLen(len(src)))
	encoder.Encode(dst, src)
	return dst
}

func EncodeToString(src []byte) string {
	return getEncoder().EncodeToString(src)
}

func EncodeUint64ToString(v uint64) string {
	data := make([]byte, 8)
	binary.BigEndian.PutUint64(data, v)

	i := 0
	for ; i < len(data); i++ {
		if data[i] != 0x0 {
			break
		}
	}

	return EncodeToString(data[i:])
}

const (
	InvalidEncoding = iota
	Std
	URL
	RawStd
	RawURL
)

func Guess(src []byte) int {
	var isRaw = !bytes.HasSuffix(src, []byte{'='})
	var isURL = !bytes.ContainsAny(src, "+/")
	switch {
	case isRaw && isURL:
		return RawURL
	case isURL:
		return URL
	case isRaw:
		return RawStd
	default:
		return Std
	}
}

// defaultDecoder is a Decoder that detects the encoding of the source and
// decodes it accordingly. This shouldn't really be required per the spec, but
// it exist because we have seen in the wild JWTs that are encoded using
// various versions of the base64 encoding.
type defaultDecoder struct{}

func (defaultDecoder) Decode(src []byte) ([]byte, error) {
	var enc *base64.Encoding

	switch Guess(src) {
	case RawURL:
		enc = base64.RawURLEncoding
	case URL:
		enc = base64.URLEncoding
	case RawStd:
		enc = base64.RawStdEncoding
	case Std:
		enc = base64.StdEncoding
	default:
		return nil, fmt.Errorf(`invalid encoding`)
	}

	dst := make([]byte, enc.DecodedLen(len(src)))
	n, err := enc.Decode(dst, src)
	if err != nil {
		return nil, fmt.Errorf(`failed to decode source: %w`, err)
	}
	return dst[:n], nil
}

func Decode(src []byte) ([]byte, error) {
	return getDecoder().Decode(src)
}

func DecodeString(src string) ([]byte, error) {
	return getDecoder().Decode([]byte(src))
}
