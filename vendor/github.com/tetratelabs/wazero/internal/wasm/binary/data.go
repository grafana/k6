package binary

import (
	"bytes"
	"fmt"
	"io"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/leb128"
	"github.com/tetratelabs/wazero/internal/wasm"
)

// dataSegmentPrefix represents three types of data segments.
//
// https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/binary/modules.html#data-section
type dataSegmentPrefix = uint32

const (
	// dataSegmentPrefixActive is the prefix for the version 1.0 compatible data segment, which is classified as "active" in 2.0.
	dataSegmentPrefixActive dataSegmentPrefix = 0x0
	// dataSegmentPrefixPassive prefixes the "passive" data segment as in version 2.0 specification.
	dataSegmentPrefixPassive dataSegmentPrefix = 0x1
	// dataSegmentPrefixActiveWithMemoryIndex is the active prefix with memory index encoded which is defined for futur use as of 2.0.
	dataSegmentPrefixActiveWithMemoryIndex dataSegmentPrefix = 0x2
)

func decodeDataSegment(r *bytes.Reader, enabledFeatures api.CoreFeatures) (*wasm.DataSegment, error) {
	dataSegmentPrefx, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read data segment prefix: %w", err)
	}

	if dataSegmentPrefx != dataSegmentPrefixActive {
		if err := enabledFeatures.RequireEnabled(api.CoreFeatureBulkMemoryOperations); err != nil {
			return nil, fmt.Errorf("non-zero prefix for data segment is invalid as %w", err)
		}
	}

	var expr *wasm.ConstantExpression
	switch dataSegmentPrefx {
	case dataSegmentPrefixActive,
		dataSegmentPrefixActiveWithMemoryIndex:
		// Active data segment as in
		// https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/binary/modules.html#data-section
		if dataSegmentPrefx == 0x2 {
			d, _, err := leb128.DecodeUint32(r)
			if err != nil {
				return nil, fmt.Errorf("read memory index: %v", err)
			} else if d != 0 {
				return nil, fmt.Errorf("memory index must be zero but was %d", d)
			}
		}

		expr, err = decodeConstantExpression(r, enabledFeatures)
		if err != nil {
			return nil, fmt.Errorf("read offset expression: %v", err)
		}
	case dataSegmentPrefixPassive:
		// Passive data segment doesn't need const expr nor memory index encoded.
		// https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/binary/modules.html#data-section
	default:
		return nil, fmt.Errorf("invalid data segment prefix: 0x%x", dataSegmentPrefx)
	}

	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get the size of vector: %v", err)
	}

	b := make([]byte, vs)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, fmt.Errorf("read bytes for init: %v", err)
	}

	return &wasm.DataSegment{
		OffsetExpression: expr,
		Init:             b,
	}, nil
}

func encodeDataSegment(d *wasm.DataSegment) (ret []byte) {
	// Currently multiple memories are not supported.
	ret = append(ret, leb128.EncodeInt32(0)...)
	ret = append(ret, encodeConstantExpression(d.OffsetExpression)...)
	ret = append(ret, leb128.EncodeUint32(uint32(len(d.Init)))...)
	ret = append(ret, d.Init...)
	return
}
