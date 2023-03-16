package binary

import (
	"github.com/tetratelabs/wazero/internal/wasm"
)

var sizePrefixedName = []byte{4, 'n', 'a', 'm', 'e'}

// EncodeModule implements wasm.EncodeModule for the WebAssembly 1.0 (20191205) Binary Format.
// Note: If saving to a file, the conventional extension is wasm
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#binary-format%E2%91%A0
func EncodeModule(m *wasm.Module) (bytes []byte) {
	bytes = append(Magic, version...)
	if m.SectionElementCount(wasm.SectionIDType) > 0 {
		bytes = append(bytes, encodeTypeSection(m.TypeSection)...)
	}
	if m.SectionElementCount(wasm.SectionIDImport) > 0 {
		bytes = append(bytes, encodeImportSection(m.ImportSection)...)
	}
	if m.SectionElementCount(wasm.SectionIDFunction) > 0 {
		bytes = append(bytes, encodeFunctionSection(m.FunctionSection)...)
	}
	if m.SectionElementCount(wasm.SectionIDTable) > 0 {
		bytes = append(bytes, encodeTableSection(m.TableSection)...)
	}
	if m.SectionElementCount(wasm.SectionIDMemory) > 0 {
		bytes = append(bytes, encodeMemorySection(m.MemorySection)...)
	}
	if m.SectionElementCount(wasm.SectionIDGlobal) > 0 {
		bytes = append(bytes, encodeGlobalSection(m.GlobalSection)...)
	}
	if m.SectionElementCount(wasm.SectionIDExport) > 0 {
		bytes = append(bytes, encodeExportSection(m.ExportSection)...)
	}
	if m.SectionElementCount(wasm.SectionIDStart) > 0 {
		bytes = append(bytes, encodeStartSection(*m.StartSection)...)
	}
	if m.SectionElementCount(wasm.SectionIDElement) > 0 {
		bytes = append(bytes, encodeElementSection(m.ElementSection)...)
	}
	if m.SectionElementCount(wasm.SectionIDCode) > 0 {
		bytes = append(bytes, encodeCodeSection(m.CodeSection)...)
	}
	if m.SectionElementCount(wasm.SectionIDData) > 0 {
		bytes = append(bytes, encodeDataSection(m.DataSection)...)
	}
	if m.SectionElementCount(wasm.SectionIDCustom) > 0 {
		// >> The name section should appear only once in a module, and only after the data section.
		// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#binary-namesec
		if m.NameSection != nil {
			nameSection := append(sizePrefixedName, encodeNameSectionData(m.NameSection)...)
			bytes = append(bytes, encodeSection(wasm.SectionIDCustom, nameSection)...)
		}
	}
	return
}
