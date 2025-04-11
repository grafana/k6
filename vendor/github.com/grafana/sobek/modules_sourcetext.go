package sobek

import (
	"fmt"
	"sort"
	"sync"

	"github.com/grafana/sobek/ast"
	"github.com/grafana/sobek/parser"
)

var (
	_ CyclicModuleRecord   = &SourceTextModuleRecord{}
	_ CyclicModuleInstance = &SourceTextModuleInstance{}
)

type SourceTextModuleInstance struct {
	moduleRecord *SourceTextModuleRecord
	// TODO figure out omething less idiotic
	exportGetters map[string]func() Value
	pcap          *promiseCapability
	asyncPromise  *Promise
}

func (s *SourceTextModuleInstance) ExecuteModule(rt *Runtime, res, rej func(interface{}) error) (CyclicModuleInstance, error) {
	promiseP := s.pcap.promise.self.(*Promise)
	if len(promiseP.fulfillReactions) == 1 {
		ar := promiseP.fulfillReactions[0].asyncRunner
		_ = ar.onFulfilled(FunctionCall{Arguments: []Value{_undefined}})
	}

	promise := s.asyncPromise
	if !s.HasTLA() {
		if res != nil {
			panic("sobek bug where a not async module was executed as async on")
		}
		// we handle the failure below so we need to mark this as promise as handled so it doesn't
		// trigger the PromiseRejectionTracker
		rt.performPromiseThen(s.asyncPromise, rt.ToValue(func() {}), rt.ToValue(func() {}), nil)
		switch s.asyncPromise.state {
		case PromiseStateFulfilled:
			return s, nil
		case PromiseStateRejected:
			return nil, rt.vm.exceptionFromValue(promise.result)
		case PromiseStatePending:
			panic("sobek bug where an sync module was not executed synchronously")
		default:
			panic("Somehow promise from a module execution is in invalid state")
		}
	}
	if res == nil {
		panic("sobek bug where an async module was not executed as async")
	}
	rt.performPromiseThen(s.asyncPromise, rt.ToValue(func(call FunctionCall) Value {
		err := res(call.Argument(0))
		if err != nil {
			panic(err)
		}
		return nil
	}), rt.ToValue(func(call FunctionCall) Value {
		v := call.Argument(0)
		err := rej(rt.vm.exceptionFromValue(v))
		if err != nil {
			panic(err)
		}
		return nil
	}), nil)
	return nil, nil
}

func (s *SourceTextModuleInstance) GetBindingValue(name string) Value {
	getter, ok := s.exportGetters[name]
	if !ok { // let's not panic in case somebody asks for a binding that isn't exported
		return nil
	}
	return getter()
}

func (s *SourceTextModuleInstance) HasTLA() bool {
	return s.moduleRecord.hasTLA
}

type SourceTextModuleRecord struct {
	body *ast.Program
	p    *Program
	// context
	// importmeta
	hasTLA                bool
	requestedModules      []string
	importEntries         []importEntry
	localExportEntries    []exportEntry
	indirectExportEntries []exportEntry
	starExportEntries     []exportEntry

	hostResolveImportedModule HostResolveImportedModuleFunc

	once *sync.Once
}

type importEntry struct {
	moduleRequest string
	importName    string
	localName     string
	offset        int
}

type exportEntry struct {
	exportName    string
	moduleRequest string
	importName    string
	localName     string
	offset        int

	// not standard
	lex bool
}

func importEntriesFromAst(declarations []*ast.ImportDeclaration) ([]importEntry, error) {
	var result []importEntry
	names := make(map[string]struct{}, len(declarations))
	for _, importDeclarion := range declarations {
		importClause := importDeclarion.ImportClause
		if importDeclarion.FromClause == nil {
			continue // no entry in this case
		}
		moduleRequest := importDeclarion.FromClause.ModuleSpecifier.String()
		if named := importClause.NamedImports; named != nil {
			for _, el := range named.ImportsList {
				localName := el.Alias.String()
				if localName == "" {
					localName = el.IdentifierName.String()
				}
				if _, ok := names[localName]; ok {
					return nil, fmt.Errorf("duplicate bounded name %s", localName)
				}
				names[localName] = struct{}{}
				result = append(result, importEntry{
					moduleRequest: moduleRequest,
					importName:    el.IdentifierName.String(),
					localName:     localName,
					offset:        int(importDeclarion.Idx0()),
				})
			}
		}
		if def := importClause.ImportedDefaultBinding; def != nil {
			localName := def.Name.String()
			if _, ok := names[localName]; ok {
				return nil, fmt.Errorf("duplicate bounded name %s", localName)
			}
			names[localName] = struct{}{}
			result = append(result, importEntry{
				moduleRequest: moduleRequest,
				importName:    "default",
				localName:     localName,
				offset:        int(importDeclarion.Idx0()),
			})
		}
		if namespace := importClause.NameSpaceImport; namespace != nil {
			localName := namespace.ImportedBinding.String()
			if _, ok := names[localName]; ok {
				return nil, fmt.Errorf("duplicate bounded name %s", localName)
			}
			names[localName] = struct{}{}
			result = append(result, importEntry{
				moduleRequest: moduleRequest,
				importName:    "*",
				localName:     namespace.ImportedBinding.String(),
				offset:        int(importDeclarion.Idx0()),
			})
		}
	}
	return result, nil
}

func exportEntryFromIdentifier(id *ast.Identifier, lex bool) exportEntry {
	name := id.Name.String()
	return exportEntry{localName: name, exportName: name, lex: lex}
}

func exportEntriesFromObjectPatter(op *ast.ObjectPattern, lex bool) []exportEntry {
	result := make([]exportEntry, 0, len(op.Properties))
	for _, p := range op.Properties {
		switch p := p.(type) {
		case *ast.PropertyShort:
			name := p.Name.Name.String()
			result = append(result, exportEntry{
				localName:  name,
				exportName: name,
				lex:        lex,
				offset:     int(op.Idx0()),
			})
		case *ast.PropertyKeyed:
			panic("exported of keyed destructuring is not supported at this time.")
		case *ast.SpreadElement:
			panic("exported of spread element destructuring is not supported at this time.")
		default:
			panic("exported of destructing with unknown type is not supported at this time.")
		}
	}
	return result
}

func exportEntriesFromAst(declarations []*ast.ExportDeclaration) []exportEntry {
	var result []exportEntry
	for _, exportDeclaration := range declarations {
		if exportDeclaration.ExportFromClause != nil {
			exportFromClause := exportDeclaration.ExportFromClause
			if namedExports := exportFromClause.NamedExports; namedExports != nil {
				for _, spec := range namedExports.ExportsList {
					result = append(result, exportEntry{
						localName:  spec.IdentifierName.String(),
						exportName: spec.Alias.String(),
						offset:     int(exportDeclaration.Idx0()),
					})
				}
			} else if exportFromClause.IsWildcard {
				if from := exportDeclaration.FromClause; from != nil {
					result = append(result, exportEntry{
						exportName:    exportFromClause.Alias.String(),
						importName:    "*",
						moduleRequest: from.ModuleSpecifier.String(),
						offset:        int(exportDeclaration.Idx0()),
					})
				} else {
					result = append(result, exportEntry{
						exportName: exportFromClause.Alias.String(),
						importName: "*",
						offset:     int(exportDeclaration.Idx0()),
					})
				}
			} else {
				panic("wat")
			}
		} else if variableDeclaration := exportDeclaration.Variable; variableDeclaration != nil {
			for _, l := range variableDeclaration.List {
				switch i := l.Target.(type) {
				case *ast.Identifier:
					result = append(result, exportEntryFromIdentifier(i, false))
				case *ast.ObjectPattern:
					result = append(result, exportEntriesFromObjectPatter(i, false)...)
				default:
					panic("target for variable declaration export isn't supported. this is sobek bug.")
				}
			}
		} else if LexicalDeclaration := exportDeclaration.LexicalDeclaration; LexicalDeclaration != nil {
			for _, l := range LexicalDeclaration.List {
				switch i := l.Target.(type) {
				case *ast.Identifier:
					result = append(result, exportEntryFromIdentifier(i, true))
				case *ast.ObjectPattern:
					result = append(result, exportEntriesFromObjectPatter(i, true)...)
				default:
					panic("target for lexical declaration export isn't supported. this is sobek bug.")
				}
			}
		} else if hoistable := exportDeclaration.HoistableDeclaration; hoistable != nil {
			localName := "default"
			exportName := "default"
			if hoistable.FunctionDeclaration != nil {
				if hoistable.FunctionDeclaration.Function.Name != nil {
					localName = string(hoistable.FunctionDeclaration.Function.Name.Name.String())
				}
			}
			if !exportDeclaration.IsDefault {
				exportName = localName
			}
			result = append(result, exportEntry{
				localName:  localName,
				exportName: exportName,
				lex:        true,
				offset:     int(exportDeclaration.Idx0()),
			})
		} else if fromClause := exportDeclaration.FromClause; fromClause != nil {
			if namedExports := exportDeclaration.NamedExports; namedExports != nil {
				for _, spec := range namedExports.ExportsList {
					alias := spec.IdentifierName.String()
					if spec.Alias != "" {
						alias = spec.Alias.String()
					}
					result = append(result, exportEntry{
						importName:    spec.IdentifierName.String(),
						exportName:    alias,
						moduleRequest: fromClause.ModuleSpecifier.String(),
						offset:        int(exportDeclaration.Idx0()),
					})
				}
			} else {
				panic("wat")
			}
		} else if namedExports := exportDeclaration.NamedExports; namedExports != nil {
			for _, spec := range namedExports.ExportsList {
				alias := spec.IdentifierName.String()
				if spec.Alias != "" {
					alias = spec.Alias.String()
				}
				result = append(result, exportEntry{
					localName:  spec.IdentifierName.String(),
					exportName: alias,
					offset:     int(exportDeclaration.Idx0()),
				})
			}
		} else if exportDeclaration.AssignExpression != nil {
			result = append(result, exportEntry{
				exportName: "default",
				localName:  "default",
				lex:        true,
				offset:     int(exportDeclaration.Idx0()),
			})
		} else if exportDeclaration.ClassDeclaration != nil {
			cls := exportDeclaration.ClassDeclaration.Class
			if exportDeclaration.IsDefault {
				localName := "default"
				if cls.Name != nil {
					localName = cls.Name.Name.String()
				}
				result = append(result, exportEntry{
					exportName: "default",
					localName:  localName,
					lex:        true,
					offset:     int(exportDeclaration.Idx0()),
				})
			} else {
				result = append(result, exportEntry{
					exportName: cls.Name.Name.String(),
					localName:  cls.Name.Name.String(),
					lex:        true,
					offset:     int(exportDeclaration.Idx0()),
				})
			}
		} else {
			panic("wat")
		}
	}
	return result
}

func requestedModulesFromAst(statements []ast.Statement) []string {
	var result []string
	for _, st := range statements {
		switch imp := st.(type) {
		case *ast.ImportDeclaration:
			if imp.FromClause != nil {
				result = append(result, imp.FromClause.ModuleSpecifier.String())
			} else {
				result = append(result, imp.ModuleSpecifier.String())
			}
		case *ast.ExportDeclaration:
			if imp.FromClause != nil {
				result = append(result, imp.FromClause.ModuleSpecifier.String())
			}
		}
	}
	return result
}

func findImportByLocalName(importEntries []importEntry, name string) (importEntry, bool) {
	for _, i := range importEntries {
		if i.localName == name {
			return i, true
		}
	}

	return importEntry{}, false
}

// This should probably be part of Parse
// TODO arguments to this need fixing
func ParseModule(name, sourceText string, resolveModule HostResolveImportedModuleFunc, opts ...parser.Option) (*SourceTextModuleRecord, error) {
	// TODO asserts
	opts = append(opts, parser.IsModule)
	body, err := Parse(name, sourceText, opts...)
	_ = body
	if err != nil {
		return nil, err
	}
	return ModuleFromAST(body, resolveModule)
}

func ModuleFromAST(body *ast.Program, resolveModule HostResolveImportedModuleFunc) (*SourceTextModuleRecord, error) {
	requestedModules := requestedModulesFromAst(body.Body)
	importEntries, err := importEntriesFromAst(body.ImportEntries)
	if err != nil {
		// TODO create a separate error type
		return nil, &CompilerSyntaxError{CompilerError: CompilerError{
			Message: err.Error(),
		}}
	}
	// 6. Let importedBoundNames be ImportedLocalNames(importEntries).
	// ^ is skipped as we don't need it.

	var indirectExportEntries []exportEntry
	var localExportEntries []exportEntry
	var starExportEntries []exportEntry
	exportEntries := exportEntriesFromAst(body.ExportEntries)
	for _, ee := range exportEntries {
		if ee.moduleRequest == "" { // technically nil
			ie, ok := findImportByLocalName(importEntries, ee.localName)
			if !ok {
				localExportEntries = append(localExportEntries, ee)
				continue
			}
			if ie.importName == "*" {
				localExportEntries = append(localExportEntries, ee)
			} else {
				indirectExportEntries = append(indirectExportEntries, exportEntry{
					moduleRequest: ie.moduleRequest,
					importName:    ie.importName,
					exportName:    ee.exportName,
				})
			}
		} else {
			if ee.importName == "*" && ee.exportName == "" {
				starExportEntries = append(starExportEntries, ee)
			} else {
				indirectExportEntries = append(indirectExportEntries, ee)
			}
		}
	}

	s := &SourceTextModuleRecord{
		// realm isn't implement
		// environment is undefined
		// namespace is undefined
		hasTLA:           body.HasTLA,
		requestedModules: requestedModules,
		// hostDefined TODO
		body: body,
		// Context empty
		// importMeta empty
		importEntries:         importEntries,
		localExportEntries:    localExportEntries,
		indirectExportEntries: indirectExportEntries,
		starExportEntries:     starExportEntries,

		hostResolveImportedModule: resolveModule,
		once:                      &sync.Once{},
	}

	names := s.getExportedNamesWithotStars() // we use this as the other one loops but wee need to early errors here
	sort.Strings(names)
	for i := 1; i < len(names); i++ {
		if names[i] == names[i-1] {
			return nil, &CompilerSyntaxError{
				CompilerError: CompilerError{
					Message: fmt.Sprintf("Duplicate export name %s", names[i]),
				},
			}
		}
		// TODO other checks
	}

	return s, nil
}

func (module *SourceTextModuleRecord) getExportedNamesWithotStars() []string {
	exportedNames := make([]string, 0, len(module.localExportEntries)+len(module.indirectExportEntries))
	for _, e := range module.localExportEntries {
		exportedNames = append(exportedNames, e.exportName)
	}
	for _, e := range module.indirectExportEntries {
		exportedNames = append(exportedNames, e.exportName)
	}
	return exportedNames
}

func (module *SourceTextModuleRecord) GetExportedNames(callback func([]string), exportStarSet ...ModuleRecord) bool {
	for _, el := range exportStarSet {
		if el == module { // better check
			// TODO assert
			callback(nil)
			return true
		}
	}
	exportStarSet = append(exportStarSet, module)
	var exportedNames []string
	for _, e := range module.localExportEntries {
		exportedNames = append(exportedNames, e.exportName)
	}
	for _, e := range module.indirectExportEntries {
		exportedNames = append(exportedNames, e.exportName)
	}
	if len(module.starExportEntries) == 0 {
		callback(exportedNames)
		return true
	}

	for i, e := range module.starExportEntries {
		requestedModule, err := module.hostResolveImportedModule(module, e.moduleRequest)
		if err != nil {
			panic(err)
		}
		ch := make(chan struct{})
		newCallback := func(names []string) {
			for _, n := range names {
				if n != "default" {
					// TODO check if n i exportedNames and don't include it
					exportedNames = append(exportedNames, n)
				}
			}
			close(ch)
		}

		isSync := requestedModule.GetExportedNames(newCallback, exportStarSet...)
		if !isSync {
			go func() {
				<-ch
				module.handleAsyncGeteExportNames(exportedNames, module.starExportEntries[i:], callback, exportStarSet...)
			}()
			return false
		}
	}
	callback(exportedNames)
	return true
}

func (module *SourceTextModuleRecord) handleAsyncGeteExportNames(
	exportedNames []string, remaining []exportEntry, callback func([]string), exportStarSet ...ModuleRecord,
) {
	for _, e := range remaining {
		requestedModule, err := module.hostResolveImportedModule(module, e.moduleRequest)
		if err != nil {
			panic(err)
		}
		ch := make(chan struct{})
		newCallback := func(names []string) {
			for _, n := range names {
				if n != "default" {
					// TODO check if n i exportedNames and don't include it
					exportedNames = append(exportedNames, n)
				}
			}
			close(ch)
		}

		_ = requestedModule.GetExportedNames(newCallback, exportStarSet...)
		<-ch
	}
	callback(exportedNames)
}

func (module *SourceTextModuleRecord) InitializeEnvironment() (err error) {
	module.once.Do(func() {
		c := newCompiler()
		defer func() {
			if x := recover(); x != nil {
				switch x1 := x.(type) {
				case *CompilerSyntaxError:
					err = x1
				default:
					panic(x)
				}
			}
		}()

		c.compileModule(module)
		module.p = c.p
	})
	return
}

type ResolveSetElement struct {
	Module     ModuleRecord
	ExportName string
}

type ResolvedBinding struct {
	Module      ModuleRecord
	BindingName string
}

// GetModuleInstance returns an instance of an already instanciated module.
// If the ModuleRecord was not instanciated at this time it will return nil
func (r *Runtime) GetModuleInstance(m ModuleRecord) ModuleInstance {
	return r.modules[m]
}

func (module *SourceTextModuleRecord) ResolveExport(exportName string, resolveset ...ResolveSetElement) (*ResolvedBinding, bool) {
	// TODO this whole algorithm can likely be used for not source module records a well
	if exportName == "" {
		panic("wat")
	}
	for _, r := range resolveset {
		if r.Module == module && exportName == r.ExportName { // TODO better
			return nil, false
		}
	}
	resolveset = append(resolveset, ResolveSetElement{Module: module, ExportName: exportName})
	for _, e := range module.localExportEntries {
		if exportName == e.exportName {
			// ii. ii. Return ResolvedBinding Record { [[Module]]: module, [[BindingName]]: e.[[LocalName]] }.
			return &ResolvedBinding{
				Module:      module,
				BindingName: e.localName,
			}, false
		}
	}

	for _, e := range module.indirectExportEntries {
		if exportName == e.exportName {
			importedModule, err := module.hostResolveImportedModule(module, e.moduleRequest)
			if err != nil {
				panic(err) // TODO return err
			}
			if e.importName == "*" {
				// 2. 2. Return ResolvedBinding Record { [[Module]]: importedModule, [[BindingName]]: "*namespace*" }.
				return &ResolvedBinding{
					Module:      importedModule,
					BindingName: "*namespace*",
				}, false
			} else {
				return importedModule.ResolveExport(e.importName, resolveset...)
			}
		}
	}
	if exportName == "default" {
		// This actually should've been caught above, but as it didn't it actually makes it s so the `default` export
		// doesn't resolve anything that is `export * ...`
		return nil, false
	}
	var starResolution *ResolvedBinding

	for _, e := range module.starExportEntries {
		importedModule, err := module.hostResolveImportedModule(module, e.moduleRequest)
		if err != nil {
			panic(err) // TODO return err
		}
		resolution, ambiguous := importedModule.ResolveExport(exportName, resolveset...)
		if ambiguous {
			return nil, true
		}
		if resolution != nil {
			if starResolution == nil {
				starResolution = resolution
			} else if resolution.Module != starResolution.Module || resolution.BindingName != starResolution.BindingName {
				return nil, true
			}
		}
	}
	return starResolution, false
}

func (module *SourceTextModuleRecord) Instantiate(rt *Runtime) (CyclicModuleInstance, error) {
	// fmt.Println("Instantiate", module.p.src.Name())
	mi := &SourceTextModuleInstance{
		moduleRecord:  module,
		exportGetters: make(map[string]func() Value),
		pcap:          rt.newPromiseCapability(rt.getPromise()),
	}
	rt.modules[module] = mi
	rt.vm.callStack = append(rt.vm.callStack, context{})
	_, ex := rt.RunProgram(module.p)
	rt.vm.callStack = rt.vm.callStack[:len(rt.vm.callStack)-1]
	if ex != nil {
		mi.pcap.reject(rt.ToValue(ex))
		return nil, ex
	}

	return mi, nil
}

func (module *SourceTextModuleRecord) Evaluate(rt *Runtime) *Promise {
	return rt.CyclicModuleRecordEvaluate(module, module.hostResolveImportedModule)
}

func (module *SourceTextModuleRecord) Link() error {
	c := newCompiler()
	c.hostResolveImportedModule = module.hostResolveImportedModule
	return c.CyclicModuleRecordConcreteLink(module)
}

func (module *SourceTextModuleRecord) RequestedModules() []string {
	return module.requestedModules
}
