package wasm

import (
	"fmt"
	"math"
	"sync"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/leb128"
)

// Table describes the limits of elements and its type in a table.
type Table struct {
	Min  uint32
	Max  *uint32
	Type RefType
}

// RefType is either RefTypeFuncref or RefTypeExternref as of WebAssembly core 2.0.
type RefType = byte

const (
	// RefTypeFuncref represents a reference to a function.
	RefTypeFuncref = ValueTypeFuncref
	// RefTypeExternref represents a reference to a host object, which is not currently supported in wazero.
	RefTypeExternref = ValueTypeExternref
)

func RefTypeName(t RefType) (ret string) {
	switch t {
	case RefTypeFuncref:
		ret = "funcref"
	case RefTypeExternref:
		ret = "externref"
	default:
		ret = fmt.Sprintf("unknown(0x%x)", t)
	}
	return
}

// ElementMode represents a mode of element segment which is either active, passive or declarative.
//
// https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/syntax/modules.html#element-segments
type ElementMode = byte

const (
	// ElementModeActive is the mode which requires the runtime to initialize table with the contents in .Init field combined with OffsetExpr.
	ElementModeActive ElementMode = iota
	// ElementModePassive is the mode which doesn't require the runtime to initialize table, and only used with OpcodeTableInitName.
	ElementModePassive
	// ElementModeDeclarative is introduced in reference-types proposal which can be used to declare function indexes used by OpcodeRefFunc.
	ElementModeDeclarative
)

// ElementSegment are initialization instructions for a TableInstance
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#syntax-elem
type ElementSegment struct {
	// OffsetExpr returns the table element offset to apply to Init indices.
	// Note: This can be validated prior to instantiation unless it includes OpcodeGlobalGet (an imported global).
	// Note: This is only set when Mode is active.
	OffsetExpr *ConstantExpression

	// TableIndex is the table's index to which this element segment is applied.
	// Note: This is used if and only if the Mode is active.
	TableIndex Index

	// Followings are set/used regardless of the Mode.

	// Init indices are (nullable) table elements where each index is the function index by which the module initialize the table.
	Init []*Index

	// Type holds the type of this element segment, which is the RefType in WebAssembly 2.0.
	Type RefType

	// Mode is the mode of this element segment.
	Mode ElementMode
}

// IsActive returns true if the element segment is "active" mode which requires the runtime to initialize table
// with the contents in .Init field.
func (e *ElementSegment) IsActive() bool {
	return e.Mode == ElementModeActive
}

// TableInstance represents a table of (RefTypeFuncref) elements in a module.
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#table-instances%E2%91%A0
type TableInstance struct {
	// References holds references whose type is either RefTypeFuncref or RefTypeExternref (unsupported).
	//
	// Currently, only function references are supported.
	References []Reference

	// Min is the minimum (function) elements in this table and cannot grow to accommodate ElementSegment.
	Min uint32

	// Max if present is the maximum (function) elements in this table, or nil if unbounded.
	Max *uint32

	// Type is either RefTypeFuncref or RefTypeExternRef.
	Type RefType

	// mux is used to prevent overlapping calls to Grow.
	mux sync.RWMutex
}

// ElementInstance represents an element instance in a module.
//
// See https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/exec/runtime.html#element-instances
type ElementInstance struct {
	// References holds references whose type is either RefTypeFuncref or RefTypeExternref (unsupported).
	References []Reference
	// Type is the RefType of the references in this instance's References.
	Type RefType
}

// Reference is the runtime representation of RefType which is either RefTypeFuncref or RefTypeExternref.
type Reference = uintptr

// validatedActiveElementSegment is like ElementSegment of active mode except the inputs are expanded and validated based on defining module.
//
// Note: The global imported at globalIdx may have an offset value that is out-of-bounds for the corresponding table.
type validatedActiveElementSegment struct {
	// opcode is OpcodeGlobalGet or OpcodeI32Const
	opcode Opcode

	// arg is the only argument to opcode, which when applied results in the offset to add to init indices.
	//  * OpcodeGlobalGet: position in the global index of an imported Global ValueTypeI32 holding the offset.
	//  * OpcodeI32Const: a constant ValueTypeI32 offset.
	arg uint32

	// init are a range of table elements whose values are positions in the function index. This range
	// replaces any values in TableInstance.Table at an offset arg which is a constant if opcode == OpcodeI32Const or
	// derived from a globalIdx if opcode == OpcodeGlobalGet
	init []*Index

	// tableIndex is the table's index to which this active element will be applied.
	tableIndex Index
}

// validateTable ensures any ElementSegment is valid. This caches results via Module.validatedActiveElementSegments.
// Note: limitsType are validated by decoders, so not re-validated here.
func (m *Module) validateTable(enabledFeatures api.CoreFeatures, tables []*Table, maximumTableIndex uint32) ([]*validatedActiveElementSegment, error) {
	if len(tables) > int(maximumTableIndex) {
		return nil, fmt.Errorf("too many tables in a module: %d given with limit %d", len(tables), maximumTableIndex)
	}

	if m.validatedActiveElementSegments != nil {
		return m.validatedActiveElementSegments, nil
	}

	importedTableCount := m.ImportTableCount()

	ret := make([]*validatedActiveElementSegment, 0, m.SectionElementCount(SectionIDElement))

	// Create bounds checks as these can err prior to instantiation
	funcCount := m.importCount(ExternTypeFunc) + m.SectionElementCount(SectionIDFunction)

	// Now, we have to figure out which table elements can be resolved before instantiation and also fail early if there
	// are any imported globals that are known to be invalid by their declarations.
	for i, elem := range m.ElementSection {
		idx := Index(i)
		initCount := uint32(len(elem.Init))

		if elem.Type == RefTypeFuncref {
			// Any offset applied is to the element, not the function index: validate here if the funcidx is sound.
			for ei, funcIdx := range elem.Init {
				if funcIdx != nil && *funcIdx >= funcCount {
					return nil, fmt.Errorf("%s[%d].init[%d] funcidx %d out of range", SectionIDName(SectionIDElement), idx, ei, *funcIdx)
				}
			}
		} else {
			for j, elem := range elem.Init {
				if elem != nil {
					return nil, fmt.Errorf("%s[%d].init[%d] must be ref.null but was %v", SectionIDName(SectionIDElement), idx, j, *elem)
				}
			}
		}

		if elem.IsActive() {
			if len(tables) <= int(elem.TableIndex) {
				return nil, fmt.Errorf("unknown table %d as active element target", elem.TableIndex)
			}

			t := tables[elem.TableIndex]
			if t.Type != elem.Type {
				return nil, fmt.Errorf("element type mismatch: table has %s but element has %s",
					RefTypeName(t.Type), RefTypeName(elem.Type),
				)
			}

			// global.get needs to be discovered during initialization
			oc := elem.OffsetExpr.Opcode
			if oc == OpcodeGlobalGet {
				globalIdx, _, err := leb128.LoadUint32(elem.OffsetExpr.Data)
				if err != nil {
					return nil, fmt.Errorf("%s[%d] couldn't read global.get parameter: %w", SectionIDName(SectionIDElement), idx, err)
				} else if err = m.verifyImportGlobalI32(SectionIDElement, idx, globalIdx); err != nil {
					return nil, err
				}

				if initCount == 0 {
					continue // Per https://github.com/WebAssembly/spec/issues/1427 init can be no-op, but validate anyway!
				}

				ret = append(ret, &validatedActiveElementSegment{opcode: oc, arg: globalIdx, init: elem.Init, tableIndex: elem.TableIndex})
			} else if oc == OpcodeI32Const {
				// Treat constants as signed as their interpretation is not yet known per /RATIONALE.md
				o, _, err := leb128.LoadInt32(elem.OffsetExpr.Data)
				if err != nil {
					return nil, fmt.Errorf("%s[%d] couldn't read i32.const parameter: %w", SectionIDName(SectionIDElement), idx, err)
				}
				offset := Index(o)

				// Per https://github.com/WebAssembly/spec/blob/wg-1.0/test/core/elem.wast#L117 we must pass if imported
				// table has set its min=0. Per https://github.com/WebAssembly/spec/blob/wg-1.0/test/core/elem.wast#L142, we
				// have to do fail if module-defined min=0.
				if !enabledFeatures.IsEnabled(api.CoreFeatureReferenceTypes) && elem.TableIndex >= importedTableCount {
					if err = checkSegmentBounds(t.Min, uint64(initCount)+uint64(offset), idx); err != nil {
						return nil, err
					}
				}

				if initCount == 0 {
					continue // Per https://github.com/WebAssembly/spec/issues/1427 init can be no-op, but validate anyway!
				}

				ret = append(ret, &validatedActiveElementSegment{opcode: oc, arg: offset, init: elem.Init, tableIndex: elem.TableIndex})
			} else {
				return nil, fmt.Errorf("%s[%d] has an invalid const expression: %s", SectionIDName(SectionIDElement), idx, InstructionName(oc))
			}
		}
	}

	m.validatedActiveElementSegments = ret
	return ret, nil
}

// buildTable returns TableInstances if the module defines or imports a table.
//   - importedTables: returned as `tables` unmodified.
//   - importedGlobals: include all instantiated, imported globals.
//
// If the result `init` is non-nil, it is the `tableInit` parameter of Engine.NewModuleEngine.
//
// Note: An error is only possible when an ElementSegment.OffsetExpr is out of range of the TableInstance.Min.
func (m *Module) buildTables(importedTables []*TableInstance, importedGlobals []*GlobalInstance, skipBoundCheck bool) (tables []*TableInstance, inits []tableInitEntry, err error) {
	tables = importedTables

	for _, tsec := range m.TableSection {
		// The module defining the table is the one that sets its Min/Max etc.
		tables = append(tables, &TableInstance{
			References: make([]Reference, tsec.Min), Min: tsec.Min, Max: tsec.Max,
			Type: tsec.Type,
		})
	}

	elementSegments := m.validatedActiveElementSegments
	if len(elementSegments) == 0 {
		return
	}

	for elemI, elem := range elementSegments {
		table := tables[elem.tableIndex]
		var offset uint32
		if elem.opcode == OpcodeGlobalGet {
			global := importedGlobals[elem.arg]
			offset = uint32(global.Val)
		} else {
			offset = elem.arg // constant
		}

		// Check to see if we are out-of-bounds
		initCount := uint64(len(elem.init))
		if !skipBoundCheck {
			if err = checkSegmentBounds(table.Min, uint64(offset)+initCount, Index(elemI)); err != nil {
				return
			}
		}

		if table.Type == RefTypeExternref {
			inits = append(inits, tableInitEntry{
				tableIndex: elem.tableIndex, offset: offset,
				// ExternRef elements are guaranteed to be all null via the validation phase.
				nullExternRefCount: len(elem.init),
			})
		} else {
			inits = append(inits, tableInitEntry{
				tableIndex: elem.tableIndex, offset: offset, functionIndexes: elem.init,
			})
		}
	}
	return
}

// tableInitEntry is normalized element segment used for initializing tables.
type tableInitEntry struct {
	tableIndex Index
	// offset is the offset in the table from which the table is initialized by engine.
	offset Index
	// functionIndexes contains nullable function indexes. This is set when the target table has RefTypeFuncref.
	functionIndexes []*Index
	// nullExternRefCount is the number of nul reference which is the only available RefTypeExternref value in elements as of
	// WebAssembly 2.0. This is set when the target table has RefTypeExternref.
	nullExternRefCount int
}

// checkSegmentBounds fails if the capacity needed for an ElementSegment.Init is larger than limitsType.Min
//
// WebAssembly 1.0 (20191205) doesn't forbid growing to accommodate element segments, and spectests are inconsistent.
// For example, the spectests enforce elements within Table limitsType.Min, but ignore Import.DescTable min. What this
// means is we have to delay offset checks on imported tables until we link to them.
// e.g. https://github.com/WebAssembly/spec/blob/wg-1.0/test/core/elem.wast#L117 wants pass on min=0 for import
// e.g. https://github.com/WebAssembly/spec/blob/wg-1.0/test/core/elem.wast#L142 wants fail on min=0 module-defined
func checkSegmentBounds(min uint32, requireMin uint64, idx Index) error { // uint64 in case offset was set to -1
	if requireMin > uint64(min) {
		return fmt.Errorf("%s[%d].init exceeds min table size", SectionIDName(SectionIDElement), idx)
	}
	return nil
}

func (m *Module) verifyImportGlobalI32(sectionID SectionID, sectionIdx Index, idx uint32) error {
	ig := uint32(math.MaxUint32) // +1 == 0
	for i, im := range m.ImportSection {
		if im.Type == ExternTypeGlobal {
			ig++
			if ig == idx {
				if im.DescGlobal.ValType != ValueTypeI32 {
					return fmt.Errorf("%s[%d] (global.get %d): import[%d].global.ValType != i32", SectionIDName(sectionID), sectionIdx, idx, i)
				}
				return nil
			}
		}
	}
	return fmt.Errorf("%s[%d] (global.get %d): out of range of imported globals", SectionIDName(sectionID), sectionIdx, idx)
}

// Grow appends the `initialRef` by `delta` times into the References slice.
// Returns -1 if the operation is not valid, otherwise the old length of the table.
//
// https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/exec/instructions.html#xref-syntax-instructions-syntax-instr-table-mathsf-table-grow-x
func (t *TableInstance) Grow(delta uint32, initialRef Reference) (currentLen uint32) {
	// We take write-lock here as the following might result in a new slice
	t.mux.Lock()
	defer t.mux.Unlock()

	currentLen = uint32(len(t.References))
	if delta == 0 {
		return
	}

	if newLen := int64(currentLen) + int64(delta); // adding as 64bit ints to avoid overflow.
	newLen >= math.MaxUint32 || (t.Max != nil && newLen > int64(*t.Max)) {
		return 0xffffffff // = -1 in signed 32-bit integer.
	}
	t.References = append(t.References, make([]uintptr, delta)...)

	// Uses the copy trick for faster filling the new region with the initial value.
	// https://gist.github.com/taylorza/df2f89d5f9ab3ffd06865062a4cf015d
	newRegion := t.References[currentLen:]
	newRegion[0] = initialRef
	for i := 1; i < len(newRegion); i *= 2 {
		copy(newRegion[i:], newRegion[:i])
	}
	return
}
