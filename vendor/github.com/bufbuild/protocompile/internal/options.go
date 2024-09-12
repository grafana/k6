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
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/bufbuild/protocompile/ast"
	"github.com/bufbuild/protocompile/reporter"
)

type hasOptionNode interface {
	OptionNode(part *descriptorpb.UninterpretedOption) ast.OptionDeclNode
	FileNode() ast.FileDeclNode // needed in order to query for NodeInfo
}

func FindFirstOption(res hasOptionNode, handler *reporter.Handler, scope string, opts []*descriptorpb.UninterpretedOption, name string) (int, error) {
	return findOption(res, handler, scope, opts, name, false, true)
}

func FindOption(res hasOptionNode, handler *reporter.Handler, scope string, opts []*descriptorpb.UninterpretedOption, name string) (int, error) {
	return findOption(res, handler, scope, opts, name, true, false)
}

func findOption(res hasOptionNode, handler *reporter.Handler, scope string, opts []*descriptorpb.UninterpretedOption, name string, exact, first bool) (int, error) {
	found := -1
	for i, opt := range opts {
		if exact && len(opt.Name) != 1 {
			continue
		}
		if opt.Name[0].GetIsExtension() || opt.Name[0].GetNamePart() != name {
			continue
		}
		if first {
			return i, nil
		}
		if found >= 0 {
			optNode := res.OptionNode(opt)
			fn := res.FileNode()
			node := optNode.GetName()
			nodeInfo := fn.NodeInfo(node)
			return -1, handler.HandleErrorf(nodeInfo, "%s: option %s cannot be defined more than once", scope, name)
		}
		found = i
	}
	return found, nil
}

func RemoveOption(uo []*descriptorpb.UninterpretedOption, indexToRemove int) []*descriptorpb.UninterpretedOption {
	switch {
	case indexToRemove == 0:
		return uo[1:]
	case indexToRemove == len(uo)-1:
		return uo[:len(uo)-1]
	default:
		return append(uo[:indexToRemove], uo[indexToRemove+1:]...)
	}
}
