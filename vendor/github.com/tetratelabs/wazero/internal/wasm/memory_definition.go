package wasm

import "github.com/tetratelabs/wazero/api"

// ImportedMemories implements the same method as documented on wazero.CompiledModule.
func (m *Module) ImportedMemories() (ret []api.MemoryDefinition) {
	for _, d := range m.MemoryDefinitionSection {
		if d.importDesc != nil {
			ret = append(ret, d)
		}
	}
	return
}

// ExportedMemories implements the same method as documented on wazero.CompiledModule.
func (m *Module) ExportedMemories() map[string]api.MemoryDefinition {
	ret := map[string]api.MemoryDefinition{}
	for _, d := range m.MemoryDefinitionSection {
		for _, e := range d.exportNames {
			ret[e] = d
		}
	}
	return ret
}

// BuildMemoryDefinitions generates memory metadata that can be parsed from
// the module. This must be called after all validation.
//
// Note: This is exported for wazero.Runtime `CompileModule`.
func (m *Module) BuildMemoryDefinitions() {
	var moduleName string
	if m.NameSection != nil {
		moduleName = m.NameSection.ModuleName
	}

	memoryCount := m.ImportMemoryCount()
	if m.MemorySection != nil {
		memoryCount++
	}

	if memoryCount == 0 {
		return
	}

	m.MemoryDefinitionSection = make([]*MemoryDefinition, 0, memoryCount)
	importMemIdx := Index(0)
	for _, i := range m.ImportSection {
		if i.Type != ExternTypeMemory {
			continue
		}

		m.MemoryDefinitionSection = append(m.MemoryDefinitionSection, &MemoryDefinition{
			importDesc: &[2]string{i.Module, i.Name},
			index:      importMemIdx,
			memory:     i.DescMem,
		})
		importMemIdx++
	}

	if m.MemorySection != nil {
		m.MemoryDefinitionSection = append(m.MemoryDefinitionSection, &MemoryDefinition{
			index:  importMemIdx,
			memory: m.MemorySection,
		})
	}

	for _, d := range m.MemoryDefinitionSection {
		d.moduleName = moduleName
		for _, e := range m.ExportSection {
			if e.Type == ExternTypeMemory && e.Index == d.index {
				d.exportNames = append(d.exportNames, e.Name)
			}
		}
	}
}

// MemoryDefinition implements api.MemoryDefinition
type MemoryDefinition struct {
	moduleName  string
	index       Index
	importDesc  *[2]string
	exportNames []string
	memory      *Memory
}

// ModuleName implements the same method as documented on api.MemoryDefinition.
func (f *MemoryDefinition) ModuleName() string {
	return f.moduleName
}

// Index implements the same method as documented on api.MemoryDefinition.
func (f *MemoryDefinition) Index() uint32 {
	return f.index
}

// Import implements the same method as documented on api.MemoryDefinition.
func (f *MemoryDefinition) Import() (moduleName, name string, isImport bool) {
	if importDesc := f.importDesc; importDesc != nil {
		moduleName, name, isImport = importDesc[0], importDesc[1], true
	}
	return
}

// ExportNames implements the same method as documented on api.MemoryDefinition.
func (f *MemoryDefinition) ExportNames() []string {
	return f.exportNames
}

// Min implements the same method as documented on api.MemoryDefinition.
func (f *MemoryDefinition) Min() uint32 {
	return f.memory.Min
}

// Max implements the same method as documented on api.MemoryDefinition.
func (f *MemoryDefinition) Max() (max uint32, encoded bool) {
	max = f.memory.Max
	encoded = f.memory.IsMaxEncoded
	return
}
