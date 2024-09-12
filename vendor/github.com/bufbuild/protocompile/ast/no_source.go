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

package ast

// UnknownPos is a placeholder position when only the source file
// name is known.
func UnknownPos(filename string) SourcePos {
	return SourcePos{Filename: filename}
}

// UnknownSpan is a placeholder span when only the source file
// name is known.
func UnknownSpan(filename string) SourceSpan {
	return unknownSpan{filename: filename}
}

type unknownSpan struct {
	filename string
}

func (s unknownSpan) Start() SourcePos {
	return UnknownPos(s.filename)
}

func (s unknownSpan) End() SourcePos {
	return UnknownPos(s.filename)
}

// NoSourceNode is a placeholder AST node that implements numerous
// interfaces in this package. It can be used to represent an AST
// element for a file whose source is not available.
type NoSourceNode struct {
	filename string
}

// NewNoSourceNode creates a new NoSourceNode for the given filename.
func NewNoSourceNode(filename string) NoSourceNode {
	return NoSourceNode{filename: filename}
}

func (n NoSourceNode) Name() string {
	return n.filename
}

func (n NoSourceNode) Start() Token {
	return 0
}

func (n NoSourceNode) End() Token {
	return 0
}

func (n NoSourceNode) NodeInfo(Node) NodeInfo {
	return NodeInfo{
		fileInfo: &FileInfo{name: n.filename},
	}
}

func (n NoSourceNode) GetSyntax() Node {
	return n
}

func (n NoSourceNode) GetName() Node {
	return n
}

func (n NoSourceNode) GetValue() ValueNode {
	return n
}

func (n NoSourceNode) FieldLabel() Node {
	return n
}

func (n NoSourceNode) FieldName() Node {
	return n
}

func (n NoSourceNode) FieldType() Node {
	return n
}

func (n NoSourceNode) FieldTag() Node {
	return n
}

func (n NoSourceNode) FieldExtendee() Node {
	return n
}

func (n NoSourceNode) GetGroupKeyword() Node {
	return n
}

func (n NoSourceNode) GetOptions() *CompactOptionsNode {
	return nil
}

func (n NoSourceNode) RangeStart() Node {
	return n
}

func (n NoSourceNode) RangeEnd() Node {
	return n
}

func (n NoSourceNode) GetNumber() Node {
	return n
}

func (n NoSourceNode) MessageName() Node {
	return n
}

func (n NoSourceNode) OneofName() Node {
	return n
}

func (n NoSourceNode) GetInputType() Node {
	return n
}

func (n NoSourceNode) GetOutputType() Node {
	return n
}

func (n NoSourceNode) Value() interface{} {
	return nil
}

func (n NoSourceNode) RangeOptions(func(*OptionNode) bool) {
}
