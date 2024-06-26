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

// Package linker contains logic and APIs related to linking a protobuf file.
// The process of linking involves resolving all symbol references to the
// referenced descriptor. The result of linking is a "rich" descriptor that
// is more useful than just a descriptor proto since the links allow easy
// traversal of a protobuf type schema and the relationships between elements.
//
// # Files
//
// This package uses an augmentation to protoreflect.FileDescriptor instances
// in the form of the File interface. There are also factory functions for
// promoting a FileDescriptor into a linker.File. This new interface provides
// additional methods for resolving symbols in the file.
//
// This interface is both the result of linking but also an input to the linking
// process, as all dependencies of a file to be linked must be provided in this
// form. The actual result of the Link function, a Result, is an even broader
// interface than File: The linker.Result interface provides even more functions,
// which are needed for subsequent compilation steps: interpreting options and
// generating source code info.
//
// # Symbols
//
// This package has a type named Symbols which represents a symbol table. This
// is usually an internal detail when linking, but callers can provide an
// instance so that symbols across multiple compile/link operations all have
// access to the same table. This allows for detection of cases where multiple
// files try to declare elements with conflicting fully-qualified names or
// declare extensions for a particular extendable message that have conflicting
// tag numbers.
//
// The calling code simply uses the same Symbols instance across all compile
// operations and if any files processed have such conflicts, they can be
// reported.
package linker
