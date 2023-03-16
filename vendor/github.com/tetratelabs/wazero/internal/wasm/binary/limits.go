package binary

import (
	"bytes"
	"fmt"

	"github.com/tetratelabs/wazero/internal/leb128"
)

// decodeLimitsType returns the `limitsType` (min, max) decoded with the WebAssembly 1.0 (20191205) Binary Format.
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#limits%E2%91%A6
func decodeLimitsType(r *bytes.Reader) (min uint32, max *uint32, err error) {
	var flag byte
	if flag, err = r.ReadByte(); err != nil {
		err = fmt.Errorf("read leading byte: %v", err)
		return
	}

	switch flag {
	case 0x00:
		min, _, err = leb128.DecodeUint32(r)
		if err != nil {
			err = fmt.Errorf("read min of limit: %v", err)
		}
	case 0x01:
		min, _, err = leb128.DecodeUint32(r)
		if err != nil {
			err = fmt.Errorf("read min of limit: %v", err)
			return
		}
		var m uint32
		if m, _, err = leb128.DecodeUint32(r); err != nil {
			err = fmt.Errorf("read max of limit: %v", err)
		} else {
			max = &m
		}
	default:
		err = fmt.Errorf("%v for limits: %#x != 0x00 or 0x01", ErrInvalidByte, flag)
	}
	return
}

// encodeLimitsType returns the `limitsType` (min, max) encoded in WebAssembly 1.0 (20191205) Binary Format.
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#limits%E2%91%A6
func encodeLimitsType(min uint32, max *uint32) []byte {
	if max == nil {
		return append(leb128.EncodeUint32(0x00), leb128.EncodeUint32(min)...)
	}
	return append(leb128.EncodeUint32(0x01), append(leb128.EncodeUint32(min), leb128.EncodeUint32(*max)...)...)
}
