//go:build jwx_asmbase64

package base64

import (
	"fmt"

	asmbase64 "github.com/segmentio/asm/base64"
)

func init() {
	SetEncoder(asmbase64.RawURLEncoding)
	SetDecoder(asmDecoder{})
}

type asmDecoder struct{}

func (d asmDecoder) Decode(src []byte) ([]byte, error) {
	var enc *asmbase64.Encoding
	switch Guess(src) {
	case Std:
		enc = asmbase64.StdEncoding
	case RawStd:
		enc = asmbase64.RawStdEncoding
	case URL:
		enc = asmbase64.URLEncoding
	case RawURL:
		enc = asmbase64.RawURLEncoding
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
