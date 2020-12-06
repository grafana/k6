package protoparse

import (
	"bytes"
	"strings"

	"github.com/golang/protobuf/proto"
	dpb "github.com/golang/protobuf/protoc-gen-go/descriptor"

	"github.com/jhump/protoreflect/desc/internal"
)

func (r *parseResult) generateSourceCodeInfo() *dpb.SourceCodeInfo {
	if r.nodes == nil {
		// skip files that do not have AST info (these will be files
		// that came from well-known descriptors, instead of from source)
		return nil
	}

	sci := sourceCodeInfo{commentsUsed: map[*comment]struct{}{}}
	path := make([]int32, 0, 10)

	fn := r.getFileNode(r.fd).(*fileNode)
	sci.newLocWithoutComments(fn, nil)

	if fn.syntax != nil {
		sci.newLoc(fn.syntax, append(path, internal.File_syntaxTag))
	}

	var depIndex, optIndex, msgIndex, enumIndex, extendIndex, svcIndex int32

	for _, child := range fn.decls {
		switch {
		case child.imp != nil:
			sci.newLoc(child.imp, append(path, internal.File_dependencyTag, int32(depIndex)))
			depIndex++
		case child.pkg != nil:
			sci.newLoc(child.pkg, append(path, internal.File_packageTag))
		case child.option != nil:
			r.generateSourceCodeInfoForOption(&sci, child.option, false, &optIndex, append(path, internal.File_optionsTag))
		case child.message != nil:
			r.generateSourceCodeInfoForMessage(&sci, child.message, nil, append(path, internal.File_messagesTag, msgIndex))
			msgIndex++
		case child.enum != nil:
			r.generateSourceCodeInfoForEnum(&sci, child.enum, append(path, internal.File_enumsTag, enumIndex))
			enumIndex++
		case child.extend != nil:
			r.generateSourceCodeInfoForExtensions(&sci, child.extend, &extendIndex, &msgIndex, append(path, internal.File_extensionsTag), append(dup(path), internal.File_messagesTag))
		case child.service != nil:
			r.generateSourceCodeInfoForService(&sci, child.service, append(path, internal.File_servicesTag, svcIndex))
			svcIndex++
		}
	}

	return &dpb.SourceCodeInfo{Location: sci.locs}
}

func (r *parseResult) generateSourceCodeInfoForOption(sci *sourceCodeInfo, n *optionNode, compact bool, uninterpIndex *int32, path []int32) {
	if !compact {
		sci.newLocWithoutComments(n, path)
	}
	subPath := r.interpretedOptions[n]
	if len(subPath) > 0 {
		p := path
		if subPath[0] == -1 {
			// used by "default" and "json_name" field pseudo-options
			// to attribute path to parent element (since those are
			// stored directly on the descriptor, not its options)
			p = make([]int32, len(path)-1)
			copy(p, path)
			subPath = subPath[1:]
		}
		sci.newLoc(n, append(p, subPath...))
		return
	}

	// it's an uninterpreted option
	optPath := append(path, internal.UninterpretedOptionsTag, *uninterpIndex)
	*uninterpIndex++
	sci.newLoc(n, optPath)
	var valTag int32
	switch n.val.(type) {
	case *compoundIdentNode:
		valTag = internal.Uninterpreted_identTag
	case *intLiteralNode:
		valTag = internal.Uninterpreted_posIntTag
	case *compoundIntNode:
		valTag = internal.Uninterpreted_negIntTag
	case *compoundFloatNode:
		valTag = internal.Uninterpreted_doubleTag
	case *compoundStringNode:
		valTag = internal.Uninterpreted_stringTag
	case *aggregateLiteralNode:
		valTag = internal.Uninterpreted_aggregateTag
	}
	if valTag != 0 {
		sci.newLoc(n.val, append(optPath, valTag))
	}
	for j, nn := range n.name.parts {
		optNmPath := append(optPath, internal.Uninterpreted_nameTag, int32(j))
		sci.newLoc(nn, optNmPath)
		sci.newLoc(nn.text, append(optNmPath, internal.UninterpretedName_nameTag))
	}
}

func (r *parseResult) generateSourceCodeInfoForMessage(sci *sourceCodeInfo, n msgDecl, fieldPath []int32, path []int32) {
	sci.newLoc(n, path)

	var decls []*messageElement
	switch n := n.(type) {
	case *messageNode:
		decls = n.decls
	case *groupNode:
		decls = n.decls
	case *mapFieldNode:
		// map entry so nothing else to do
		return
	}

	sci.newLoc(n.messageName(), append(path, internal.Message_nameTag))
	// matching protoc, which emits the corresponding field type name (for group fields)
	// right after the source location for the group message name
	if fieldPath != nil {
		sci.newLoc(n.messageName(), append(fieldPath, internal.Field_typeNameTag))
	}

	var optIndex, fieldIndex, oneOfIndex, extendIndex, nestedMsgIndex int32
	var nestedEnumIndex, extRangeIndex, reservedRangeIndex, reservedNameIndex int32
	for _, child := range decls {
		switch {
		case child.option != nil:
			r.generateSourceCodeInfoForOption(sci, child.option, false, &optIndex, append(path, internal.Message_optionsTag))
		case child.field != nil:
			r.generateSourceCodeInfoForField(sci, child.field, append(path, internal.Message_fieldsTag, fieldIndex))
			fieldIndex++
		case child.group != nil:
			fldPath := append(path, internal.Message_fieldsTag, fieldIndex)
			r.generateSourceCodeInfoForField(sci, child.group, fldPath)
			fieldIndex++
			r.generateSourceCodeInfoForMessage(sci, child.group, fldPath, append(dup(path), internal.Message_nestedMessagesTag, nestedMsgIndex))
			nestedMsgIndex++
		case child.mapField != nil:
			r.generateSourceCodeInfoForField(sci, child.mapField, append(path, internal.Message_fieldsTag, fieldIndex))
			fieldIndex++
		case child.oneOf != nil:
			r.generateSourceCodeInfoForOneOf(sci, child.oneOf, &fieldIndex, &nestedMsgIndex, append(path, internal.Message_fieldsTag), append(dup(path), internal.Message_nestedMessagesTag), append(dup(path), internal.Message_oneOfsTag, oneOfIndex))
			oneOfIndex++
		case child.nested != nil:
			r.generateSourceCodeInfoForMessage(sci, child.nested, nil, append(path, internal.Message_nestedMessagesTag, nestedMsgIndex))
			nestedMsgIndex++
		case child.enum != nil:
			r.generateSourceCodeInfoForEnum(sci, child.enum, append(path, internal.Message_enumsTag, nestedEnumIndex))
			nestedEnumIndex++
		case child.extend != nil:
			r.generateSourceCodeInfoForExtensions(sci, child.extend, &extendIndex, &nestedMsgIndex, append(path, internal.Message_extensionsTag), append(dup(path), internal.Message_nestedMessagesTag))
		case child.extensionRange != nil:
			r.generateSourceCodeInfoForExtensionRanges(sci, child.extensionRange, &extRangeIndex, append(path, internal.Message_extensionRangeTag))
		case child.reserved != nil:
			if len(child.reserved.names) > 0 {
				resPath := append(path, internal.Message_reservedNameTag)
				sci.newLoc(child.reserved, resPath)
				for _, rn := range child.reserved.names {
					sci.newLoc(rn, append(resPath, reservedNameIndex))
					reservedNameIndex++
				}
			}
			if len(child.reserved.ranges) > 0 {
				resPath := append(path, internal.Message_reservedRangeTag)
				sci.newLoc(child.reserved, resPath)
				for _, rr := range child.reserved.ranges {
					r.generateSourceCodeInfoForReservedRange(sci, rr, append(resPath, reservedRangeIndex))
					reservedRangeIndex++
				}
			}
		}
	}
}

func (r *parseResult) generateSourceCodeInfoForEnum(sci *sourceCodeInfo, n *enumNode, path []int32) {
	sci.newLoc(n, path)
	sci.newLoc(n.name, append(path, internal.Enum_nameTag))

	var optIndex, valIndex, reservedNameIndex, reservedRangeIndex int32
	for _, child := range n.decls {
		switch {
		case child.option != nil:
			r.generateSourceCodeInfoForOption(sci, child.option, false, &optIndex, append(path, internal.Enum_optionsTag))
		case child.value != nil:
			r.generateSourceCodeInfoForEnumValue(sci, child.value, append(path, internal.Enum_valuesTag, valIndex))
			valIndex++
		case child.reserved != nil:
			if len(child.reserved.names) > 0 {
				resPath := append(path, internal.Enum_reservedNameTag)
				sci.newLoc(child.reserved, resPath)
				for _, rn := range child.reserved.names {
					sci.newLoc(rn, append(resPath, reservedNameIndex))
					reservedNameIndex++
				}
			}
			if len(child.reserved.ranges) > 0 {
				resPath := append(path, internal.Enum_reservedRangeTag)
				sci.newLoc(child.reserved, resPath)
				for _, rr := range child.reserved.ranges {
					r.generateSourceCodeInfoForReservedRange(sci, rr, append(resPath, reservedRangeIndex))
					reservedRangeIndex++
				}
			}
		}
	}
}

func (r *parseResult) generateSourceCodeInfoForEnumValue(sci *sourceCodeInfo, n *enumValueNode, path []int32) {
	sci.newLoc(n, path)
	sci.newLoc(n.name, append(path, internal.EnumVal_nameTag))
	sci.newLoc(n.getNumber(), append(path, internal.EnumVal_numberTag))

	// enum value options
	if n.options != nil {
		optsPath := append(path, internal.EnumVal_optionsTag)
		sci.newLoc(n.options, optsPath)
		var optIndex int32
		for _, opt := range n.options.decls {
			r.generateSourceCodeInfoForOption(sci, opt, true, &optIndex, optsPath)
		}
	}
}

func (r *parseResult) generateSourceCodeInfoForReservedRange(sci *sourceCodeInfo, n *rangeNode, path []int32) {
	sci.newLoc(n, path)
	sci.newLoc(n.startNode, append(path, internal.ReservedRange_startTag))
	if n.endNode != nil {
		sci.newLoc(n.endNode, append(path, internal.ReservedRange_endTag))
	}
}

func (r *parseResult) generateSourceCodeInfoForExtensions(sci *sourceCodeInfo, n *extendNode, extendIndex, msgIndex *int32, extendPath, msgPath []int32) {
	sci.newLoc(n, extendPath)
	for _, decl := range n.decls {
		switch {
		case decl.field != nil:
			r.generateSourceCodeInfoForField(sci, decl.field, append(extendPath, *extendIndex))
			*extendIndex++
		case decl.group != nil:
			fldPath := append(extendPath, *extendIndex)
			r.generateSourceCodeInfoForField(sci, decl.group, fldPath)
			*extendIndex++
			r.generateSourceCodeInfoForMessage(sci, decl.group, fldPath, append(msgPath, *msgIndex))
			*msgIndex++
		}
	}
}

func (r *parseResult) generateSourceCodeInfoForOneOf(sci *sourceCodeInfo, n *oneOfNode, fieldIndex, nestedMsgIndex *int32, fieldPath, nestedMsgPath, oneOfPath []int32) {
	sci.newLoc(n, oneOfPath)
	sci.newLoc(n.name, append(oneOfPath, internal.OneOf_nameTag))

	var optIndex int32
	for _, child := range n.decls {
		switch {
		case child.option != nil:
			r.generateSourceCodeInfoForOption(sci, child.option, false, &optIndex, append(oneOfPath, internal.OneOf_optionsTag))
		case child.field != nil:
			r.generateSourceCodeInfoForField(sci, child.field, append(fieldPath, *fieldIndex))
			*fieldIndex++
		case child.group != nil:
			fldPath := append(fieldPath, *fieldIndex)
			r.generateSourceCodeInfoForField(sci, child.group, fldPath)
			*fieldIndex++
			r.generateSourceCodeInfoForMessage(sci, child.group, fldPath, append(nestedMsgPath, *nestedMsgIndex))
			*nestedMsgIndex++
		}
	}
}

func (r *parseResult) generateSourceCodeInfoForField(sci *sourceCodeInfo, n fieldDecl, path []int32) {
	isGroup := false
	var opts *compactOptionsNode
	var extendee *extendNode
	var fieldType string
	switch n := n.(type) {
	case *fieldNode:
		opts = n.options
		extendee = n.extendee
		fieldType = n.fldType.val
	case *mapFieldNode:
		opts = n.options
	case *groupNode:
		isGroup = true
		extendee = n.extendee
	case *syntheticMapField:
		// shouldn't get here since we don't recurse into fields from a mapNode
		// in generateSourceCodeInfoForMessage... but just in case
		return
	}

	if isGroup {
		// comments will appear on group message
		sci.newLocWithoutComments(n, path)
		if extendee != nil {
			sci.newLoc(extendee.extendee, append(path, internal.Field_extendeeTag))
		}
		if n.fieldLabel() != nil {
			// no comments here either (label is first token for group, so we want
			// to leave the comments to be associated with the group message instead)
			sci.newLocWithoutComments(n.fieldLabel(), append(path, internal.Field_labelTag))
		}
		sci.newLoc(n.fieldType(), append(path, internal.Field_typeTag))
		// let the name comments be attributed to the group name
		sci.newLocWithoutComments(n.fieldName(), append(path, internal.Field_nameTag))
	} else {
		sci.newLoc(n, path)
		if extendee != nil {
			sci.newLoc(extendee.extendee, append(path, internal.Field_extendeeTag))
		}
		if n.fieldLabel() != nil {
			sci.newLoc(n.fieldLabel(), append(path, internal.Field_labelTag))
		}
		n.fieldType()
		var tag int32
		if _, isScalar := fieldTypes[fieldType]; isScalar {
			tag = internal.Field_typeTag
		} else {
			// this is a message or an enum, so attribute type location
			// to the type name field
			tag = internal.Field_typeNameTag
		}
		sci.newLoc(n.fieldType(), append(path, tag))
		sci.newLoc(n.fieldName(), append(path, internal.Field_nameTag))
	}
	sci.newLoc(n.fieldTag(), append(path, internal.Field_numberTag))

	if opts != nil {
		optsPath := append(path, internal.Field_optionsTag)
		sci.newLoc(opts, optsPath)
		var optIndex int32
		for _, opt := range opts.decls {
			r.generateSourceCodeInfoForOption(sci, opt, true, &optIndex, optsPath)
		}
	}
}

func (r *parseResult) generateSourceCodeInfoForExtensionRanges(sci *sourceCodeInfo, n *extensionRangeNode, extRangeIndex *int32, path []int32) {
	sci.newLoc(n, path)
	for _, child := range n.ranges {
		path := append(path, *extRangeIndex)
		*extRangeIndex++
		sci.newLoc(child, path)
		sci.newLoc(child.startNode, append(path, internal.ExtensionRange_startTag))
		if child.endNode != nil {
			sci.newLoc(child.endNode, append(path, internal.ExtensionRange_endTag))
		}
		if n.options != nil {
			optsPath := append(path, internal.ExtensionRange_optionsTag)
			sci.newLoc(n.options, optsPath)
			var optIndex int32
			for _, opt := range n.options.decls {
				r.generateSourceCodeInfoForOption(sci, opt, true, &optIndex, optsPath)
			}
		}
	}
}

func (r *parseResult) generateSourceCodeInfoForService(sci *sourceCodeInfo, n *serviceNode, path []int32) {
	sci.newLoc(n, path)
	sci.newLoc(n.name, append(path, internal.Service_nameTag))
	var optIndex, rpcIndex int32
	for _, child := range n.decls {
		switch {
		case child.option != nil:
			r.generateSourceCodeInfoForOption(sci, child.option, false, &optIndex, append(path, internal.Service_optionsTag))
		case child.rpc != nil:
			r.generateSourceCodeInfoForMethod(sci, child.rpc, append(path, internal.Service_methodsTag, rpcIndex))
			rpcIndex++
		}
	}
}

func (r *parseResult) generateSourceCodeInfoForMethod(sci *sourceCodeInfo, n *methodNode, path []int32) {
	sci.newLoc(n, path)
	sci.newLoc(n.name, append(path, internal.Method_nameTag))
	if n.input.streamKeyword != nil {
		sci.newLoc(n.input.streamKeyword, append(path, internal.Method_inputStreamTag))
	}
	sci.newLoc(n.input.msgType, append(path, internal.Method_inputTag))
	if n.output.streamKeyword != nil {
		sci.newLoc(n.output.streamKeyword, append(path, internal.Method_outputStreamTag))
	}
	sci.newLoc(n.output.msgType, append(path, internal.Method_outputTag))

	optsPath := append(path, internal.Method_optionsTag)
	var optIndex int32
	for _, opt := range n.options {
		r.generateSourceCodeInfoForOption(sci, opt, false, &optIndex, optsPath)
	}
}

type sourceCodeInfo struct {
	locs         []*dpb.SourceCodeInfo_Location
	commentsUsed map[*comment]struct{}
}

func (sci *sourceCodeInfo) newLocWithoutComments(n node, path []int32) {
	dup := make([]int32, len(path))
	copy(dup, path)
	sci.locs = append(sci.locs, &dpb.SourceCodeInfo_Location{
		Path: dup,
		Span: makeSpan(n.start(), n.end()),
	})
}

func (sci *sourceCodeInfo) newLoc(n node, path []int32) {
	leadingComments := n.leadingComments()
	trailingComments := n.trailingComments()
	if sci.commentUsed(leadingComments) {
		leadingComments = nil
	}
	if sci.commentUsed(trailingComments) {
		trailingComments = nil
	}
	detached := groupComments(leadingComments)
	var trail *string
	if str, ok := combineComments(trailingComments); ok {
		trail = proto.String(str)
	}
	var lead *string
	if len(leadingComments) > 0 && leadingComments[len(leadingComments)-1].end.Line >= n.start().Line-1 {
		lead = proto.String(detached[len(detached)-1])
		detached = detached[:len(detached)-1]
	}
	dup := make([]int32, len(path))
	copy(dup, path)
	sci.locs = append(sci.locs, &dpb.SourceCodeInfo_Location{
		LeadingDetachedComments: detached,
		LeadingComments:         lead,
		TrailingComments:        trail,
		Path:                    dup,
		Span:                    makeSpan(n.start(), n.end()),
	})
}

func makeSpan(start, end *SourcePos) []int32 {
	if start.Line == end.Line {
		return []int32{int32(start.Line) - 1, int32(start.Col) - 1, int32(end.Col) - 1}
	}
	return []int32{int32(start.Line) - 1, int32(start.Col) - 1, int32(end.Line) - 1, int32(end.Col) - 1}
}

func (sci *sourceCodeInfo) commentUsed(c []comment) bool {
	if len(c) == 0 {
		return false
	}
	if _, ok := sci.commentsUsed[&c[0]]; ok {
		return true
	}

	sci.commentsUsed[&c[0]] = struct{}{}
	return false
}

func groupComments(comments []comment) []string {
	if len(comments) == 0 {
		return nil
	}

	var groups []string
	singleLineStyle := comments[0].text[:2] == "//"
	line := comments[0].end.Line
	start := 0
	for i := 1; i < len(comments); i++ {
		c := comments[i]
		prevSingleLine := singleLineStyle
		singleLineStyle = strings.HasPrefix(comments[i].text, "//")
		if !singleLineStyle || prevSingleLine != singleLineStyle || c.start.Line > line+1 {
			// new group!
			if str, ok := combineComments(comments[start:i]); ok {
				groups = append(groups, str)
			}
			start = i
		}
		line = c.end.Line
	}
	// don't forget last group
	if str, ok := combineComments(comments[start:]); ok {
		groups = append(groups, str)
	}
	return groups
}

func combineComments(comments []comment) (string, bool) {
	if len(comments) == 0 {
		return "", false
	}
	var buf bytes.Buffer
	for _, c := range comments {
		if c.text[:2] == "//" {
			buf.WriteString(c.text[2:])
		} else {
			lines := strings.Split(c.text[2:len(c.text)-2], "\n")
			first := true
			for _, l := range lines {
				if first {
					first = false
				} else {
					buf.WriteByte('\n')
				}

				// strip a prefix of whitespace followed by '*'
				j := 0
				for j < len(l) {
					if l[j] != ' ' && l[j] != '\t' {
						break
					}
					j++
				}
				if j == len(l) {
					l = ""
				} else if l[j] == '*' {
					l = l[j+1:]
				} else if j > 0 {
					l = " " + l[j:]
				}

				buf.WriteString(l)
			}
		}
	}
	return buf.String(), true
}

func dup(p []int32) []int32 {
	return append(([]int32)(nil), p...)
}
