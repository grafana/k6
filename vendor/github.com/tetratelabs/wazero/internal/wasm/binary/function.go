package binary

import (
	"bytes"
	"fmt"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/leb128"
	"github.com/tetratelabs/wazero/internal/wasm"
)

// encodeFunctionType returns the wasm.FunctionType encoded in WebAssembly 1.0 (20191205) Binary Format.
//
// Note: Function types are encoded by the byte 0x60 followed by the respective vectors of parameter and result types.
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#function-types%E2%91%A4
func encodeFunctionType(t *wasm.FunctionType) []byte {
	// Only reached when "multi-value" is enabled because WebAssembly 1.0 (20191205) supports at most 1 result.
	data := append([]byte{0x60}, encodeValTypes(t.Params)...)
	return append(data, encodeValTypes(t.Results)...)
}

func decodeFunctionType(enabledFeatures api.CoreFeatures, r *bytes.Reader) (*wasm.FunctionType, error) {
	b, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("read leading byte: %w", err)
	}

	if b != 0x60 {
		return nil, fmt.Errorf("%w: %#x != 0x60", ErrInvalidByte, b)
	}

	paramCount, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("could not read parameter count: %w", err)
	}

	paramTypes, err := decodeValueTypes(r, paramCount)
	if err != nil {
		return nil, fmt.Errorf("could not read parameter types: %w", err)
	}

	resultCount, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("could not read result count: %w", err)
	}

	// Guard >1.0 feature multi-value
	if resultCount > 1 {
		if err = enabledFeatures.RequireEnabled(api.CoreFeatureMultiValue); err != nil {
			return nil, fmt.Errorf("multiple result types invalid as %v", err)
		}
	}

	resultTypes, err := decodeValueTypes(r, resultCount)
	if err != nil {
		return nil, fmt.Errorf("could not read result types: %w", err)
	}

	ret := &wasm.FunctionType{
		Params:  paramTypes,
		Results: resultTypes,
	}

	// cache the key for the function type
	_ = ret.String()

	return ret, nil
}
