package wasm

import "fmt"

// ImportFuncCount returns the possibly empty count of imported functions. This plus SectionElementCount of
// SectionIDFunction is the size of the function index.
func (m *Module) ImportFuncCount() uint32 {
	return m.importCount(ExternTypeFunc)
}

// ImportTableCount returns the possibly empty count of imported tables. This plus SectionElementCount of SectionIDTable
// is the size of the table index.
func (m *Module) ImportTableCount() uint32 {
	return m.importCount(ExternTypeTable)
}

// ImportMemoryCount returns the possibly empty count of imported memories. This plus SectionElementCount of
// SectionIDMemory is the size of the memory index.
func (m *Module) ImportMemoryCount() uint32 {
	return m.importCount(ExternTypeMemory) // TODO: once validation happens on decode, this is zero or one.
}

// ImportGlobalCount returns the possibly empty count of imported globals. This plus SectionElementCount of
// SectionIDGlobal is the size of the global index.
func (m *Module) ImportGlobalCount() uint32 {
	return m.importCount(ExternTypeGlobal)
}

// importCount returns the count of a specific type of import. This is important because it is easy to mistake the
// length of the import section with the count of a specific kind of import.
func (m *Module) importCount(et ExternType) (res uint32) {
	for _, im := range m.ImportSection {
		if im.Type == et {
			res++
		}
	}
	return
}

// SectionElementCount returns the count of elements in a given section ID
//
// For example...
// * SectionIDType returns the count of FunctionType
// * SectionIDCustom returns the count of CustomSections plus one if NameSection is present
// * SectionIDHostFunction returns the count of HostFunctionSection
// * SectionIDExport returns the count of unique export names
func (m *Module) SectionElementCount(sectionID SectionID) uint32 { // element as in vector elements!
	switch sectionID {
	case SectionIDCustom:
		numCustomSections := uint32(len(m.CustomSections))
		if m.NameSection != nil {
			numCustomSections++
		}
		return numCustomSections
	case SectionIDType:
		return uint32(len(m.TypeSection))
	case SectionIDImport:
		return uint32(len(m.ImportSection))
	case SectionIDFunction:
		return uint32(len(m.FunctionSection))
	case SectionIDTable:
		return uint32(len(m.TableSection))
	case SectionIDMemory:
		if m.MemorySection != nil {
			return 1
		}
		return 0
	case SectionIDGlobal:
		return uint32(len(m.GlobalSection))
	case SectionIDExport:
		return uint32(len(m.ExportSection))
	case SectionIDStart:
		if m.StartSection != nil {
			return 1
		}
		return 0
	case SectionIDElement:
		return uint32(len(m.ElementSection))
	case SectionIDCode:
		return uint32(len(m.CodeSection))
	case SectionIDData:
		return uint32(len(m.DataSection))
	default:
		panic(fmt.Errorf("BUG: unknown section: %d", sectionID))
	}
}
