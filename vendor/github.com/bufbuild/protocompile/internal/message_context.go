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

package internal

import (
	"bytes"
	"fmt"

	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/bufbuild/protocompile/ast"
)

// ParsedFile wraps an optional AST and required FileDescriptorProto.
// This is used so types like parser.Result can be passed to this internal package avoiding circular imports.
// Additionally, it makes it less likely that users might specify one or the other.
type ParsedFile interface {
	// AST returns the parsed abstract syntax tree. This returns nil if the
	// Result was created without an AST.
	AST() *ast.FileNode
	// FileDescriptorProto returns the file descriptor proto.
	FileDescriptorProto() *descriptorpb.FileDescriptorProto
}

// MessageContext provides information about the location in a descriptor
// hierarchy, for adding context to warnings and error messages.
type MessageContext struct {
	// The relevant file
	File ParsedFile

	// The type and fully-qualified name of the element within the file.
	ElementType string
	ElementName string

	// If the element being processed is an option (or *in* an option)
	// on the named element above, this will be non-nil.
	Option *descriptorpb.UninterpretedOption
	// If the element being processed is inside a message literal in an
	// option value, this will be non-empty and represent a traversal
	// to the element in question.
	OptAggPath string
}

func (c *MessageContext) String() string {
	var ctx bytes.Buffer
	if c.ElementType != "file" {
		_, _ = fmt.Fprintf(&ctx, "%s %s: ", c.ElementType, c.ElementName)
	}
	if c.Option != nil && c.Option.Name != nil {
		ctx.WriteString("option ")
		writeOptionName(&ctx, c.Option.Name)
		if c.File.AST() == nil {
			// if we have no source position info, try to provide as much context
			// as possible (if nodes != nil, we don't need this because any errors
			// will actually have file and line numbers)
			if c.OptAggPath != "" {
				_, _ = fmt.Fprintf(&ctx, " at %s", c.OptAggPath)
			}
		}
		ctx.WriteString(": ")
	}
	return ctx.String()
}

func writeOptionName(buf *bytes.Buffer, parts []*descriptorpb.UninterpretedOption_NamePart) {
	first := true
	for _, p := range parts {
		if first {
			first = false
		} else {
			buf.WriteByte('.')
		}
		nm := p.GetNamePart()
		if nm[0] == '.' {
			// skip leading dot
			nm = nm[1:]
		}
		if p.GetIsExtension() {
			buf.WriteByte('(')
			buf.WriteString(nm)
			buf.WriteByte(')')
		} else {
			buf.WriteString(nm)
		}
	}
}
