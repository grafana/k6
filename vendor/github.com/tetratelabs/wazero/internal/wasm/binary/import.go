package binary

import (
	"bytes"
	"fmt"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/leb128"
	"github.com/tetratelabs/wazero/internal/wasm"
)

func decodeImport(
	r *bytes.Reader,
	idx uint32,
	memorySizer memorySizer,
	memoryLimitPages uint32,
	enabledFeatures api.CoreFeatures,
) (i *wasm.Import, err error) {
	i = &wasm.Import{}
	if i.Module, _, err = decodeUTF8(r, "import module"); err != nil {
		return nil, fmt.Errorf("import[%d] error decoding module: %w", idx, err)
	}

	if i.Name, _, err = decodeUTF8(r, "import name"); err != nil {
		return nil, fmt.Errorf("import[%d] error decoding name: %w", idx, err)
	}

	b, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("import[%d] error decoding type: %w", idx, err)
	}
	i.Type = b
	switch i.Type {
	case wasm.ExternTypeFunc:
		i.DescFunc, _, err = leb128.DecodeUint32(r)
	case wasm.ExternTypeTable:
		i.DescTable, err = decodeTable(r, enabledFeatures)
	case wasm.ExternTypeMemory:
		i.DescMem, err = decodeMemory(r, memorySizer, memoryLimitPages)
	case wasm.ExternTypeGlobal:
		i.DescGlobal, err = decodeGlobalType(r)
	default:
		err = fmt.Errorf("%w: invalid byte for importdesc: %#x", ErrInvalidByte, b)
	}
	if err != nil {
		return nil, fmt.Errorf("import[%d] %s[%s.%s]: %w", idx, wasm.ExternTypeName(i.Type), i.Module, i.Name, err)
	}
	return
}

// encodeImport returns the wasm.Import encoded in WebAssembly 1.0 (20191205) Binary Format.
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#binary-import
func encodeImport(i *wasm.Import) []byte {
	data := encodeSizePrefixed([]byte(i.Module))
	data = append(data, encodeSizePrefixed([]byte(i.Name))...)
	data = append(data, i.Type)
	switch i.Type {
	case wasm.ExternTypeFunc:
		data = append(data, leb128.EncodeUint32(i.DescFunc)...)
	case wasm.ExternTypeTable:
		data = append(data, wasm.RefTypeFuncref)
		data = append(data, encodeLimitsType(i.DescTable.Min, i.DescTable.Max)...)
	case wasm.ExternTypeMemory:
		maxPtr := &i.DescMem.Max
		if !i.DescMem.IsMaxEncoded {
			maxPtr = nil
		}
		data = append(data, encodeLimitsType(i.DescMem.Min, maxPtr)...)
	case wasm.ExternTypeGlobal:
		g := i.DescGlobal
		var mutable byte
		if g.Mutable {
			mutable = 1
		}
		data = append(data, g.ValType, mutable)
	default:
		panic(fmt.Errorf("invalid externtype: %s", wasm.ExternTypeName(i.Type)))
	}
	return data
}
