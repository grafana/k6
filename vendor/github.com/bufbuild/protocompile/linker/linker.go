// Copyright 2020-2024 Buf Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package linker

import (
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/bufbuild/protocompile/ast"
	"github.com/bufbuild/protocompile/parser"
	"github.com/bufbuild/protocompile/reporter"
)

// Link handles linking a parsed descriptor proto into a fully-linked descriptor.
// If the given parser.Result has imports, they must all be present in the given
// dependencies.
//
// The symbols value is optional and may be nil. If it is not nil, it must be the
// same instance used to create and link all of the given result's dependencies
// (or otherwise already have all dependencies imported). Otherwise, linking may
// fail with spurious errors resolving symbols.
//
// The handler value is used to report any link errors. If any such errors are
// reported, this function returns a non-nil error. The Result value returned
// also implements protoreflect.FileDescriptor.
//
// Note that linking does NOT interpret options. So options messages in the
// returned value have all values stored in UninterpretedOptions fields.
func Link(parsed parser.Result, dependencies Files, symbols *Symbols, handler *reporter.Handler) (Result, error) {
	if symbols == nil {
		symbols = &Symbols{}
	}
	prefix := parsed.FileDescriptorProto().GetPackage()
	if prefix != "" {
		prefix += "."
	}

	for _, imp := range parsed.FileDescriptorProto().Dependency {
		dep := dependencies.FindFileByPath(imp)
		if dep == nil {
			return nil, fmt.Errorf("dependencies is missing import %q", imp)
		}
		if err := symbols.Import(dep, handler); err != nil {
			return nil, err
		}
	}

	r := &result{
		FileDescriptor:       noOpFile,
		Result:               parsed,
		deps:                 dependencies,
		descriptors:          map[string]protoreflect.Descriptor{},
		usedImports:          map[string]struct{}{},
		prefix:               prefix,
		optionQualifiedNames: map[ast.IdentValueNode]string{},
	}

	// First, we put all symbols into a single pool, which lets us ensure there
	// are no duplicate symbols and will also let us resolve and revise all type
	// references in next step.
	if err := symbols.importResult(r, handler); err != nil {
		return nil, err
	}

	// After we've populated the pool, we can now try to resolve all type
	// references. All references must be checked for correct type, any fields
	// with enum types must be corrected (since we parse them as if they are
	// message references since we don't actually know message or enum until
	// link time), and references will be re-written to be fully-qualified
	// references (e.g. start with a dot ".").
	if err := r.resolveReferences(handler, symbols); err != nil {
		return nil, err
	}

	return r, handler.Error()
}

// Result is the result of linking. This is a protoreflect.FileDescriptor, but
// with some additional methods for exposing additional information, such as the
// for accessing the input AST or file descriptor.
//
// It also provides Resolve* methods, for looking up enums, messages, and
// extensions that are available to the protobuf source file this result
// represents. An element is "available" if it meets any of the following
// criteria:
//  1. The element is defined in this file itself.
//  2. The element is defined in a file that is directly imported by this file.
//  3. The element is "available" to a file that is directly imported by this
//     file as a public import.
//
// Other elements, even if in the transitive closure of this file, are not
// available and thus won't be returned by these methods.
type Result interface {
	File
	parser.Result

	// ResolveMessageLiteralExtensionName returns the fully qualified name for
	// an identifier for extension field names in message literals.
	ResolveMessageLiteralExtensionName(ast.IdentValueNode) string
	// ValidateOptions runs some validation checks on the descriptor that can only
	// be done after options are interpreted. Any errors or warnings encountered
	// will be reported via the given handler. If any error is reported, this
	// function returns a non-nil error.
	ValidateOptions(handler *reporter.Handler, symbols *Symbols) error
	// CheckForUnusedImports is used to report warnings for unused imports. This
	// should be called after options have been interpreted. Otherwise, the logic
	// could incorrectly report imports as unused if the only symbol used were a
	// custom option.
	CheckForUnusedImports(handler *reporter.Handler)
	// PopulateSourceCodeInfo is used to populate source code info for the file
	// descriptor. This step requires that the underlying descriptor proto have
	// its `source_code_info` field populated. This is typically a post-process
	// step separate from linking, because computing source code info requires
	// interpreting options (which is done after linking).
	PopulateSourceCodeInfo()

	// RemoveAST drops the AST information from this result.
	RemoveAST()
}

// ErrorUnusedImport may be passed to a warning reporter when an unused
// import is detected. The error the reporter receives will be wrapped
// with source position that indicates the file and line where the import
// statement appeared.
type ErrorUnusedImport interface {
	error
	UnusedImport() string
}

type errUnusedImport string

func (e errUnusedImport) Error() string {
	return fmt.Sprintf("import %q not used", string(e))
}

func (e errUnusedImport) UnusedImport() string {
	return string(e)
}
