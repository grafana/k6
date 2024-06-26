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

// Package protocompile provides the entry point for a high performance
// native Go protobuf compiler. "Compile" in this case just means parsing
// and validating source and generating fully-linked descriptors in the end.
// Unlike the protoc command-line tool, this package does not try to use the
// descriptors to perform code generation.
//
// The various sub-packages represent the various compile phases and contain
// models for the intermediate results. Those phases follow:
//  1. Parse into AST.
//     Also see: parser.Parse
//  2. Convert AST to unlinked descriptor protos.
//     Also see: parser.ResultFromAST
//  3. Link descriptor protos into "rich" descriptors.
//     Also see: linker.Link
//  4. Interpret custom options.
//     Also see: options.InterpretOptions
//  5. Generate source code info.
//     Also see: sourceinfo.GenerateSourceInfo
//
// This package provides an easy-to-use interface that does all the relevant
// phases, based on the inputs given. If an input is provided as source, all
// phases apply. If an input is provided as a descriptor proto, only phases
// 3 to 5 apply. Nothing is necessary if provided a linked descriptor (which
// is usually only the case for select system dependencies).
//
// This package is also capable of taking advantage of multiple CPU cores, so
// a compilation involving thousands of files can be done very quickly by
// compiling things in parallel.
//
// # Resolvers
//
// A Resolver is how the compiler locates artifacts that are inputs to the
// compilation. For example, it can load protobuf source code that must be
// processed. A Resolver could also supply some already-compiled dependencies
// as fully-linked descriptors, alleviating the need to re-compile them.
//
// A Resolver can provide any of the following in response to a query for an
// input.
//   - Source code: If a resolver answers a query with protobuf source, the
//     compiler will parse and compile it.
//   - AST: If a resolver answers a query with an AST, the parsing step can be
//     skipped, and the rest of the compilation steps will be applied.
//   - Descriptor proto: If a resolver answers a query with an unlinked proto,
//     only the other compilation steps, including linking, need to be applied.
//   - Descriptor: If a resolver answers a query with a fully-linked descriptor,
//     nothing further needs to be done. The descriptor is used as-is.
//
// Compilation will use the Resolver to load the files that are to be compiled
// and also to load all dependencies (i.e. other files imported by those being
// compiled).
//
// # Compiler
//
// A Compiler accepts a list of file names and produces the list of descriptors.
// A Compiler has several fields that control how it works but only the Resolver
// field is required. A minimal Compiler, that resolves files by loading them
// from the file system based on the current working directory, can be had with
// the following simple snippet:
//
//	compiler := protocompile.Compiler{
//	    Resolver: &protocompile.SourceResolver{},
//	}
//
// This minimal Compiler will use default parallelism, equal to the number of
// CPU cores detected; it will not generate source code info in the resulting
// descriptors; and it will fail fast at the first sign of any error. All of
// these aspects can be customized by setting other fields.
package protocompile
