package binary

import (
	"bytes"
	"fmt"
	"io"
	"math"

	"github.com/tetratelabs/wazero/internal/leb128"
	"github.com/tetratelabs/wazero/internal/wasm"
)

func decodeCode(r *bytes.Reader, codeSectionStart uint64) (*wasm.Code, error) {
	ss, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get the size of code: %w", err)
	}
	remaining := int64(ss)

	// parse locals
	ls, bytesRead, err := leb128.DecodeUint32(r)
	remaining -= int64(bytesRead)
	if err != nil {
		return nil, fmt.Errorf("get the size locals: %v", err)
	} else if remaining < 0 {
		return nil, io.EOF
	}

	var nums []uint64
	var types []wasm.ValueType
	var sum uint64
	var n uint32
	for i := uint32(0); i < ls; i++ {
		n, bytesRead, err = leb128.DecodeUint32(r)
		remaining -= int64(bytesRead) + 1 // +1 for the subsequent ReadByte
		if err != nil {
			return nil, fmt.Errorf("read n of locals: %v", err)
		} else if remaining < 0 {
			return nil, io.EOF
		}

		sum += uint64(n)
		nums = append(nums, uint64(n))

		b, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("read type of local: %v", err)
		}
		switch vt := b; vt {
		case wasm.ValueTypeI32, wasm.ValueTypeF32, wasm.ValueTypeI64, wasm.ValueTypeF64,
			wasm.ValueTypeFuncref, wasm.ValueTypeExternref, wasm.ValueTypeV128:
			types = append(types, vt)
		default:
			return nil, fmt.Errorf("invalid local type: 0x%x", vt)
		}
	}

	if sum > math.MaxUint32 {
		return nil, fmt.Errorf("too many locals: %d", sum)
	}

	var localTypes []wasm.ValueType
	for i, num := range nums {
		t := types[i]
		for j := uint64(0); j < num; j++ {
			localTypes = append(localTypes, t)
		}
	}

	bodyOffsetInCodeSection := codeSectionStart - uint64(r.Len())
	body := make([]byte, remaining)
	if _, err = io.ReadFull(r, body); err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if endIndex := len(body) - 1; endIndex < 0 || body[endIndex] != wasm.OpcodeEnd {
		return nil, fmt.Errorf("expr not end with OpcodeEnd")
	}

	return &wasm.Code{Body: body, LocalTypes: localTypes, BodyOffsetInCodeSection: bodyOffsetInCodeSection}, nil
}

// encodeCode returns the wasm.Code encoded in WebAssembly 1.0 (20191205) Binary Format.
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#binary-code
func encodeCode(c *wasm.Code) []byte {
	if c.GoFunc != nil {
		panic("BUG: GoFunction is not encodable")
	}

	// local blocks compress locals while preserving index order by grouping locals of the same type.
	// https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#code-section%E2%91%A0
	localBlockCount := uint32(0) // how many blocks of locals with the same type (types can repeat!)
	var localBlocks []byte
	localTypeLen := len(c.LocalTypes)
	if localTypeLen > 0 {
		i := localTypeLen - 1
		var runCount uint32              // count of the same type
		var lastValueType wasm.ValueType // initialize to an invalid type 0

		// iterate backwards so it is easier to size prefix
		for ; i >= 0; i-- {
			vt := c.LocalTypes[i]
			if lastValueType != vt {
				if runCount != 0 { // Only on the first iteration, this is zero when vt is compared against invalid
					localBlocks = append(leb128.EncodeUint32(runCount), localBlocks...)
				}
				lastValueType = vt
				localBlocks = append(leb128.EncodeUint32(uint32(vt)), localBlocks...) // reuse the EncodeUint32 cache
				localBlockCount++
				runCount = 1
			} else {
				runCount++
			}
		}
		localBlocks = append(leb128.EncodeUint32(runCount), localBlocks...)
		localBlocks = append(leb128.EncodeUint32(localBlockCount), localBlocks...)
	} else {
		localBlocks = leb128.EncodeUint32(0)
	}
	code := append(localBlocks, c.Body...)
	return append(leb128.EncodeUint32(uint32(len(code))), code...)
}
