package binary

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/leb128"
	"github.com/tetratelabs/wazero/internal/wasm"
)

func ensureElementKindFuncRef(r *bytes.Reader) error {
	elemKind, err := r.ReadByte()
	if err != nil {
		return fmt.Errorf("read element prefix: %w", err)
	}
	if elemKind != 0x0 { // ElemKind is fixed to 0x0 now: https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/binary/modules.html#element-section
		return fmt.Errorf("element kind must be zero but was 0x%x", elemKind)
	}
	return nil
}

func decodeElementInitValueVector(r *bytes.Reader) ([]*wasm.Index, error) {
	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get size of vector: %w", err)
	}

	vec := make([]*wasm.Index, vs)
	for i := range vec {
		u32, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read function index: %w", err)
		}
		vec[i] = &u32
	}
	return vec, nil
}

func decodeElementConstExprVector(r *bytes.Reader, elemType wasm.RefType, enabledFeatures api.CoreFeatures) ([]*wasm.Index, error) {
	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("failed to get the size of constexpr vector: %w", err)
	}
	vec := make([]*wasm.Index, vs)
	for i := range vec {
		expr, err := decodeConstantExpression(r, enabledFeatures)
		if err != nil {
			return nil, err
		}
		switch expr.Opcode {
		case wasm.OpcodeRefFunc:
			if elemType != wasm.RefTypeFuncref {
				return nil, fmt.Errorf("element type mismatch: want %s, but constexpr has funcref", wasm.RefTypeName(elemType))
			}
			v, _, _ := leb128.LoadUint32(expr.Data)
			vec[i] = &v
		case wasm.OpcodeRefNull:
			if elemType != expr.Data[0] {
				return nil, fmt.Errorf("element type mismatch: want %s, but constexpr has %s",
					wasm.RefTypeName(elemType), wasm.RefTypeName(expr.Data[0]))
			}
			// vec[i] is already nil, so nothing to do.
		default:
			return nil, fmt.Errorf("const expr must be either ref.null or ref.func but was %s", wasm.InstructionName(expr.Opcode))
		}
	}
	return vec, nil
}

func decodeElementRefType(r *bytes.Reader) (ret wasm.RefType, err error) {
	ret, err = r.ReadByte()
	if err != nil {
		err = fmt.Errorf("read element ref type: %w", err)
		return
	}
	if ret != wasm.RefTypeFuncref && ret != wasm.RefTypeExternref {
		return 0, errors.New("ref type must be funcref or externref for element as of WebAssembly 2.0")
	}
	return
}

const (
	// The prefix is explained at https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/binary/modules.html#element-section

	// elementSegmentPrefixLegacy is the legacy prefix and is only valid one before CoreFeatureBulkMemoryOperations.
	elementSegmentPrefixLegacy = iota
	// elementSegmentPrefixPassiveFuncrefValueVector is the passive element whose indexes are encoded as vec(varint), and reftype is fixed to funcref.
	elementSegmentPrefixPassiveFuncrefValueVector
	// elementSegmentPrefixActiveFuncrefValueVectorWithTableIndex is the same as elementSegmentPrefixPassiveFuncrefValueVector but active and table index is encoded.
	elementSegmentPrefixActiveFuncrefValueVectorWithTableIndex
	// elementSegmentPrefixDeclarativeFuncrefValueVector is the same as elementSegmentPrefixPassiveFuncrefValueVector but declarative.
	elementSegmentPrefixDeclarativeFuncrefValueVector
	// elementSegmentPrefixActiveFuncrefConstExprVector is active whoce reftype is fixed to funcref and indexes are encoded as vec(const_expr).
	elementSegmentPrefixActiveFuncrefConstExprVector
	// elementSegmentPrefixPassiveConstExprVector is passive whoce indexes are encoded as vec(const_expr), and reftype is encoded.
	elementSegmentPrefixPassiveConstExprVector
	// elementSegmentPrefixPassiveConstExprVector is active whoce indexes are encoded as vec(const_expr), and reftype and table index are encoded.
	elementSegmentPrefixActiveConstExprVector
	// elementSegmentPrefixDeclarativeConstExprVector is declarative whoce indexes are encoded as vec(const_expr), and reftype is encoded.
	elementSegmentPrefixDeclarativeConstExprVector
)

func decodeElementSegment(r *bytes.Reader, enabledFeatures api.CoreFeatures) (*wasm.ElementSegment, error) {
	prefix, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read element prefix: %w", err)
	}

	if prefix != elementSegmentPrefixLegacy {
		if err := enabledFeatures.RequireEnabled(api.CoreFeatureBulkMemoryOperations); err != nil {
			return nil, fmt.Errorf("non-zero prefix for element segment is invalid as %w", err)
		}
	}

	// Encoding depends on the prefix and described at https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/binary/modules.html#element-section
	switch prefix {
	case elementSegmentPrefixLegacy:
		// Legacy prefix which is WebAssembly 1.0 compatible.
		expr, err := decodeConstantExpression(r, enabledFeatures)
		if err != nil {
			return nil, fmt.Errorf("read expr for offset: %w", err)
		}

		init, err := decodeElementInitValueVector(r)
		if err != nil {
			return nil, err
		}

		return &wasm.ElementSegment{
			OffsetExpr: expr,
			Init:       init,
			Type:       wasm.RefTypeFuncref,
			Mode:       wasm.ElementModeActive,
			// Legacy prefix has the fixed table index zero.
			TableIndex: 0,
		}, nil
	case elementSegmentPrefixPassiveFuncrefValueVector:
		// Prefix 1 requires funcref.
		if err = ensureElementKindFuncRef(r); err != nil {
			return nil, err
		}

		init, err := decodeElementInitValueVector(r)
		if err != nil {
			return nil, err
		}
		return &wasm.ElementSegment{
			Init: init,
			Type: wasm.RefTypeFuncref,
			Mode: wasm.ElementModePassive,
		}, nil
	case elementSegmentPrefixActiveFuncrefValueVectorWithTableIndex:
		tableIndex, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("get size of vector: %w", err)
		}

		if tableIndex != 0 {
			if err := enabledFeatures.RequireEnabled(api.CoreFeatureReferenceTypes); err != nil {
				return nil, fmt.Errorf("table index must be zero but was %d: %w", tableIndex, err)
			}
		}

		expr, err := decodeConstantExpression(r, enabledFeatures)
		if err != nil {
			return nil, fmt.Errorf("read expr for offset: %w", err)
		}

		// Prefix 2 requires funcref.
		if err = ensureElementKindFuncRef(r); err != nil {
			return nil, err
		}

		init, err := decodeElementInitValueVector(r)
		if err != nil {
			return nil, err
		}
		return &wasm.ElementSegment{
			OffsetExpr: expr,
			Init:       init,
			Type:       wasm.RefTypeFuncref,
			Mode:       wasm.ElementModeActive,
			TableIndex: tableIndex,
		}, nil
	case elementSegmentPrefixDeclarativeFuncrefValueVector:
		// Prefix 3 requires funcref.
		if err = ensureElementKindFuncRef(r); err != nil {
			return nil, err
		}
		init, err := decodeElementInitValueVector(r)
		if err != nil {
			return nil, err
		}
		return &wasm.ElementSegment{
			Init: init,
			Type: wasm.RefTypeFuncref,
			Mode: wasm.ElementModeDeclarative,
		}, nil
	case elementSegmentPrefixActiveFuncrefConstExprVector:
		expr, err := decodeConstantExpression(r, enabledFeatures)
		if err != nil {
			return nil, fmt.Errorf("read expr for offset: %w", err)
		}

		init, err := decodeElementConstExprVector(r, wasm.RefTypeFuncref, enabledFeatures)
		if err != nil {
			return nil, err
		}

		return &wasm.ElementSegment{
			OffsetExpr: expr,
			Init:       init,
			Type:       wasm.RefTypeFuncref,
			Mode:       wasm.ElementModeActive,
			TableIndex: 0,
		}, nil
	case elementSegmentPrefixPassiveConstExprVector:
		refType, err := decodeElementRefType(r)
		if err != nil {
			return nil, err
		}
		init, err := decodeElementConstExprVector(r, refType, enabledFeatures)
		if err != nil {
			return nil, err
		}
		return &wasm.ElementSegment{
			Init: init,
			Type: refType,
			Mode: wasm.ElementModePassive,
		}, nil
	case elementSegmentPrefixActiveConstExprVector:
		tableIndex, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("get size of vector: %w", err)
		}

		if tableIndex != 0 {
			if err := enabledFeatures.RequireEnabled(api.CoreFeatureReferenceTypes); err != nil {
				return nil, fmt.Errorf("table index must be zero but was %d: %w", tableIndex, err)
			}
		}
		expr, err := decodeConstantExpression(r, enabledFeatures)
		if err != nil {
			return nil, fmt.Errorf("read expr for offset: %w", err)
		}

		refType, err := decodeElementRefType(r)
		if err != nil {
			return nil, err
		}

		init, err := decodeElementConstExprVector(r, refType, enabledFeatures)
		if err != nil {
			return nil, err
		}

		return &wasm.ElementSegment{
			OffsetExpr: expr,
			Init:       init,
			Type:       refType,
			Mode:       wasm.ElementModeActive,
			TableIndex: tableIndex,
		}, nil
	case elementSegmentPrefixDeclarativeConstExprVector:
		refType, err := decodeElementRefType(r)
		if err != nil {
			return nil, err
		}
		init, err := decodeElementConstExprVector(r, refType, enabledFeatures)
		if err != nil {
			return nil, err
		}
		return &wasm.ElementSegment{
			Init: init,
			Type: refType,
			Mode: wasm.ElementModeDeclarative,
		}, nil
	default:
		return nil, fmt.Errorf("invalid element segment prefix: 0x%x", prefix)
	}
}

// encodeCode returns the wasm.ElementSegment encoded in WebAssembly 1.0 (20191205) Binary Format.
//
// https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#element-section%E2%91%A0
func encodeElement(e *wasm.ElementSegment) (ret []byte) {
	if e.Mode == wasm.ElementModeActive {
		ret = append(ret, leb128.EncodeInt32(int32(e.TableIndex))...)
		ret = append(ret, encodeConstantExpression(e.OffsetExpr)...)
		ret = append(ret, leb128.EncodeUint32(uint32(len(e.Init)))...)
		for _, idx := range e.Init {
			ret = append(ret, leb128.EncodeInt32(int32(*idx))...)
		}
	} else {
		panic("TODO: support encoding for non-active elements in bulk-memory-operations proposal")
	}
	return
}
