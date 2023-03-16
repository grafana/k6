package wasm

import (
	"fmt"
	"sort"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/wasmdebug"
)

type HostFuncExporter interface {
	ExportHostFunc(*HostFunc)
}

// HostFunc is a function with an inlined type, used for NewHostModule.
// Any corresponding FunctionType will be reused or added to the Module.
type HostFunc struct {
	// ExportNames is equivalent to the same method on api.FunctionDefinition.
	ExportNames []string

	// Name is equivalent to the same method on api.FunctionDefinition.
	Name string

	// ParamTypes is equivalent to the same method on api.FunctionDefinition.
	ParamTypes []ValueType

	// ParamNames is equivalent to the same method on api.FunctionDefinition.
	ParamNames []string

	// ResultTypes is equivalent to the same method on api.FunctionDefinition.
	ResultTypes []ValueType

	// ResultNames is equivalent to the same method on api.FunctionDefinition.
	ResultNames []string

	// Code is the equivalent function in the SectionIDCode.
	Code *Code
}

// MustGoReflectFunc calls WithGoReflectFunc or panics on error.
func (f *HostFunc) MustGoReflectFunc(fn interface{}) *HostFunc {
	if ret, err := f.WithGoReflectFunc(fn); err != nil {
		panic(err)
	} else {
		return ret
	}
}

// WithGoFunc returns a copy of the function, replacing its Code.GoFunc.
func (f *HostFunc) WithGoFunc(fn api.GoFunc) *HostFunc {
	ret := *f
	ret.Code = &Code{IsHostFunction: true, GoFunc: fn}
	return &ret
}

// WithGoModuleFunc returns a copy of the function, replacing its Code.GoFunc.
func (f *HostFunc) WithGoModuleFunc(fn api.GoModuleFunc) *HostFunc {
	ret := *f
	ret.Code = &Code{IsHostFunction: true, GoFunc: fn}
	return &ret
}

// WithGoReflectFunc returns a copy of the function, replacing its Code.GoFunc.
func (f *HostFunc) WithGoReflectFunc(fn interface{}) (*HostFunc, error) {
	ret := *f
	var err error
	ret.ParamTypes, ret.ResultTypes, ret.Code, err = parseGoReflectFunc(fn)
	return &ret, err
}

// WithWasm returns a copy of the function, replacing its Code.Body.
func (f *HostFunc) WithWasm(body []byte) *HostFunc {
	ret := *f
	ret.Code = &Code{IsHostFunction: true, Body: body}
	if f.Code != nil {
		ret.Code.LocalTypes = f.Code.LocalTypes
	}
	return &ret
}

type HostFuncNames struct {
	Name        string
	ParamNames  []string
	ResultNames []string
}

// NewHostModule is defined internally for use in WASI tests and to keep the code size in the root directory small.
func NewHostModule(
	moduleName string,
	nameToGoFunc map[string]interface{},
	funcToNames map[string]*HostFuncNames,
	enabledFeatures api.CoreFeatures,
) (m *Module, err error) {
	if moduleName != "" {
		m = &Module{NameSection: &NameSection{ModuleName: moduleName}}
	} else {
		m = &Module{}
	}

	if exportCount := uint32(len(nameToGoFunc)); exportCount > 0 {
		m.ExportSection = make([]*Export, 0, exportCount)
		if err = addFuncs(m, nameToGoFunc, funcToNames, enabledFeatures); err != nil {
			return
		}
	}

	m.IsHostModule = true
	// Uses the address of *wasm.Module as the module ID so that host functions can have each state per compilation.
	// Downside of this is that compilation cache on host functions (trampoline codes for Go functions and
	// Wasm codes for Wasm-implemented host functions) are not available and compiles each time. On the other hand,
	// compilation of host modules is not costly as it's merely small trampolines vs the real-world native Wasm binary.
	// TODO: refactor engines so that we can properly cache compiled machine codes for host modules.
	m.AssignModuleID([]byte(fmt.Sprintf("@@@@@@@@%p", m))) // @@@@@@@@ = any 8 bytes different from Wasm header.
	m.BuildFunctionDefinitions()
	return
}

func addFuncs(
	m *Module,
	nameToGoFunc map[string]interface{},
	funcToNames map[string]*HostFuncNames,
	enabledFeatures api.CoreFeatures,
) (err error) {
	if m.NameSection == nil {
		m.NameSection = &NameSection{}
	}
	moduleName := m.NameSection.ModuleName
	nameToFunc := make(map[string]*HostFunc, len(nameToGoFunc))
	sortedExportNames := make([]string, len(nameToFunc))
	for k := range nameToGoFunc {
		sortedExportNames = append(sortedExportNames, k)
	}

	// Sort names for consistent iteration
	sort.Strings(sortedExportNames)

	funcNames := make([]string, len(nameToFunc))
	for _, k := range sortedExportNames {
		v := nameToGoFunc[k]
		if hf, ok := v.(*HostFunc); ok {
			nameToFunc[hf.Name] = hf
			funcNames = append(funcNames, hf.Name)
		} else { // reflection
			params, results, code, ftErr := parseGoReflectFunc(v)
			if ftErr != nil {
				return fmt.Errorf("func[%s.%s] %w", moduleName, k, ftErr)
			}
			hf = &HostFunc{
				ExportNames: []string{k},
				Name:        k,
				ParamTypes:  params,
				ResultTypes: results,
				Code:        code,
			}

			// Assign names to the function, if they exist.
			ns := funcToNames[k]
			if name := ns.Name; name != "" {
				hf.Name = ns.Name
			}
			if paramNames := ns.ParamNames; paramNames != nil {
				if paramNamesLen := len(paramNames); paramNamesLen != len(params) {
					return fmt.Errorf("func[%s.%s] has %d params, but %d params names", moduleName, k, paramNamesLen, len(params))
				}
				hf.ParamNames = paramNames
			}
			if resultNames := ns.ResultNames; resultNames != nil {
				if resultNamesLen := len(resultNames); resultNamesLen != len(results) {
					return fmt.Errorf("func[%s.%s] has %d results, but %d results names", moduleName, k, resultNamesLen, len(results))
				}
				hf.ResultNames = resultNames
			}

			nameToFunc[k] = hf
			funcNames = append(funcNames, k)
		}
	}

	funcCount := uint32(len(nameToFunc))
	m.NameSection.FunctionNames = make([]*NameAssoc, 0, funcCount)
	m.FunctionSection = make([]Index, 0, funcCount)
	m.CodeSection = make([]*Code, 0, funcCount)
	m.FunctionDefinitionSection = make([]*FunctionDefinition, 0, funcCount)

	idx := Index(0)
	for _, name := range funcNames {
		hf := nameToFunc[name]
		debugName := wasmdebug.FuncName(moduleName, name, idx)
		typeIdx, typeErr := m.maybeAddType(hf.ParamTypes, hf.ResultTypes, enabledFeatures)
		if typeErr != nil {
			return fmt.Errorf("func[%s] %v", debugName, typeErr)
		}
		m.FunctionSection = append(m.FunctionSection, typeIdx)
		m.CodeSection = append(m.CodeSection, hf.Code)
		for _, export := range hf.ExportNames {
			m.ExportSection = append(m.ExportSection, &Export{Type: ExternTypeFunc, Name: export, Index: idx})
		}
		m.NameSection.FunctionNames = append(m.NameSection.FunctionNames, &NameAssoc{Index: idx, Name: hf.Name})

		if len(hf.ParamNames) > 0 {
			localNames := &NameMapAssoc{Index: idx}
			for i, n := range hf.ParamNames {
				localNames.NameMap = append(localNames.NameMap, &NameAssoc{Index: Index(i), Name: n})
			}
			m.NameSection.LocalNames = append(m.NameSection.LocalNames, localNames)
		}
		if len(hf.ResultNames) > 0 {
			resultNames := &NameMapAssoc{Index: idx}
			for i, n := range hf.ResultNames {
				resultNames.NameMap = append(resultNames.NameMap, &NameAssoc{Index: Index(i), Name: n})
			}
			m.NameSection.ResultNames = append(m.NameSection.ResultNames, resultNames)
		}
		idx++
	}
	return nil
}

func (m *Module) maybeAddType(params, results []ValueType, enabledFeatures api.CoreFeatures) (Index, error) {
	if len(results) > 1 {
		// Guard >1.0 feature multi-value
		if err := enabledFeatures.RequireEnabled(api.CoreFeatureMultiValue); err != nil {
			return 0, fmt.Errorf("multiple result types invalid as %v", err)
		}
	}
	for i, t := range m.TypeSection {
		if t.EqualsSignature(params, results) {
			return Index(i), nil
		}
	}

	result := m.SectionElementCount(SectionIDType)
	toAdd := &FunctionType{Params: params, Results: results}
	m.TypeSection = append(m.TypeSection, toAdd)
	return result, nil
}
