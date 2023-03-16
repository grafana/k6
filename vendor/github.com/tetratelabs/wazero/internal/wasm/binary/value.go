package binary

import (
	"bytes"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/tetratelabs/wazero/internal/leb128"
	"github.com/tetratelabs/wazero/internal/wasm"
)

var noValType = []byte{0}

// encodedValTypes is a cache of size prefixed binary encoding of known val types.
var encodedValTypes = map[wasm.ValueType][]byte{
	wasm.ValueTypeI32:       {1, wasm.ValueTypeI32},
	wasm.ValueTypeI64:       {1, wasm.ValueTypeI64},
	wasm.ValueTypeF32:       {1, wasm.ValueTypeF32},
	wasm.ValueTypeF64:       {1, wasm.ValueTypeF64},
	wasm.ValueTypeExternref: {1, wasm.ValueTypeExternref},
	wasm.ValueTypeFuncref:   {1, wasm.ValueTypeFuncref},
	wasm.ValueTypeV128:      {1, wasm.ValueTypeV128},
}

// encodeValTypes fast paths binary encoding of common value type lengths
func encodeValTypes(vt []wasm.ValueType) []byte {
	// Special case nullary and parameter lengths of wasi_snapshot_preview1 to avoid excess allocations
	switch uint32(len(vt)) {
	case 0: // nullary
		return noValType
	case 1: // ex $wasi.fd_close or any result
		if encoded, ok := encodedValTypes[vt[0]]; ok {
			return encoded
		}
	case 2: // ex $wasi.environ_sizes_get
		return []byte{2, vt[0], vt[1]}
	case 4: // ex $wasi.fd_write
		return []byte{4, vt[0], vt[1], vt[2], vt[3]}
	case 9: // ex $wasi.fd_write
		return []byte{9, vt[0], vt[1], vt[2], vt[3], vt[4], vt[5], vt[6], vt[7], vt[8]}
	}
	// Slow path others until someone complains with a valid signature
	count := leb128.EncodeUint32(uint32(len(vt)))
	return append(count, vt...)
}

func decodeValueTypes(r *bytes.Reader, num uint32) ([]wasm.ValueType, error) {
	if num == 0 {
		return nil, nil
	}

	ret := make([]wasm.ValueType, num)
	_, err := io.ReadFull(r, ret)
	if err != nil {
		return nil, err
	}

	for _, v := range ret {
		switch v {
		case wasm.ValueTypeI32, wasm.ValueTypeF32, wasm.ValueTypeI64, wasm.ValueTypeF64,
			wasm.ValueTypeExternref, wasm.ValueTypeFuncref, wasm.ValueTypeV128:
		default:
			return nil, fmt.Errorf("invalid value type: %d", v)
		}
	}
	return ret, nil
}

// decodeUTF8 decodes a size prefixed string from the reader, returning it and the count of bytes read.
// contextFormat and contextArgs apply an error format when present
func decodeUTF8(r *bytes.Reader, contextFormat string, contextArgs ...interface{}) (string, uint32, error) {
	size, sizeOfSize, err := leb128.DecodeUint32(r)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read %s size: %w", fmt.Sprintf(contextFormat, contextArgs...), err)
	}

	buf := make([]byte, size)
	if _, err = io.ReadFull(r, buf); err != nil {
		return "", 0, fmt.Errorf("failed to read %s: %w", fmt.Sprintf(contextFormat, contextArgs...), err)
	}

	if !utf8.Valid(buf) {
		return "", 0, fmt.Errorf("%s is not valid UTF-8", fmt.Sprintf(contextFormat, contextArgs...))
	}

	return string(buf), size + uint32(sizeOfSize), nil
}
