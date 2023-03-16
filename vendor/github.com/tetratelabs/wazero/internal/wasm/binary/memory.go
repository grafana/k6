package binary

import (
	"bytes"

	"github.com/tetratelabs/wazero/internal/wasm"
)

// decodeMemory returns the api.Memory decoded with the WebAssembly 1.0 (20191205) Binary Format.
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#binary-memory
func decodeMemory(
	r *bytes.Reader,
	memorySizer memorySizer,
	memoryLimitPages uint32,
) (*wasm.Memory, error) {
	min, maxP, err := decodeLimitsType(r)
	if err != nil {
		return nil, err
	}

	min, capacity, max := memorySizer(min, maxP)
	mem := &wasm.Memory{Min: min, Cap: capacity, Max: max, IsMaxEncoded: maxP != nil}

	return mem, mem.Validate(memoryLimitPages)
}

// encodeMemory returns the wasm.Memory encoded in WebAssembly 1.0 (20191205) Binary Format.
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#binary-memory
func encodeMemory(i *wasm.Memory) []byte {
	maxPtr := &i.Max
	if !i.IsMaxEncoded {
		maxPtr = nil
	}
	return encodeLimitsType(i.Min, maxPtr)
}
