package protoparse

import (
	"bytes"
	"math"
	"strings"
	"unicode"

	"github.com/golang/protobuf/proto"
	dpb "github.com/golang/protobuf/protoc-gen-go/descriptor"

	"github.com/jhump/protoreflect/desc/internal"
)

func (r *parseResult) createFileDescriptor(filename string, file *fileNode) {
	fd := &dpb.FileDescriptorProto{Name: proto.String(filename)}
	r.fd = fd
	r.putFileNode(fd, file)

	isProto3 := false
	if file.syntax != nil {
		if file.syntax.syntax.val == "proto3" {
			isProto3 = true
		} else if file.syntax.syntax.val != "proto2" {
			if r.errs.handleErrorWithPos(file.syntax.syntax.start(), `syntax value must be "proto2" or "proto3"`) != nil {
				return
			}
		}

		// proto2 is the default, so no need to set unless proto3
		if isProto3 {
			fd.Syntax = proto.String(file.syntax.syntax.val)
		}
	} else {
		r.errs.warn(file.start(), ErrNoSyntax)
	}

	for _, decl := range file.decls {
		if r.errs.err != nil {
			return
		}
		if decl.enum != nil {
			fd.EnumType = append(fd.EnumType, r.asEnumDescriptor(decl.enum))
		} else if decl.extend != nil {
			r.addExtensions(decl.extend, &fd.Extension, &fd.MessageType, isProto3)
		} else if decl.imp != nil {
			file.imports = append(file.imports, decl.imp)
			index := len(fd.Dependency)
			fd.Dependency = append(fd.Dependency, decl.imp.name.val)
			if decl.imp.public {
				fd.PublicDependency = append(fd.PublicDependency, int32(index))
			} else if decl.imp.weak {
				fd.WeakDependency = append(fd.WeakDependency, int32(index))
			}
		} else if decl.message != nil {
			fd.MessageType = append(fd.MessageType, r.asMessageDescriptor(decl.message, isProto3))
		} else if decl.option != nil {
			if fd.Options == nil {
				fd.Options = &dpb.FileOptions{}
			}
			fd.Options.UninterpretedOption = append(fd.Options.UninterpretedOption, r.asUninterpretedOption(decl.option))
		} else if decl.service != nil {
			fd.Service = append(fd.Service, r.asServiceDescriptor(decl.service))
		} else if decl.pkg != nil {
			if fd.Package != nil {
				if r.errs.handleErrorWithPos(decl.pkg.start(), "files should have only one package declaration") != nil {
					return
				}
			}
			fd.Package = proto.String(decl.pkg.name.val)
		}
	}
}

func (r *parseResult) asUninterpretedOptions(nodes []*optionNode) []*dpb.UninterpretedOption {
	if len(nodes) == 0 {
		return nil
	}
	opts := make([]*dpb.UninterpretedOption, len(nodes))
	for i, n := range nodes {
		opts[i] = r.asUninterpretedOption(n)
	}
	return opts
}

func (r *parseResult) asUninterpretedOption(node *optionNode) *dpb.UninterpretedOption {
	opt := &dpb.UninterpretedOption{Name: r.asUninterpretedOptionName(node.name.parts)}
	r.putOptionNode(opt, node)

	switch val := node.val.value().(type) {
	case bool:
		if val {
			opt.IdentifierValue = proto.String("true")
		} else {
			opt.IdentifierValue = proto.String("false")
		}
	case int64:
		opt.NegativeIntValue = proto.Int64(val)
	case uint64:
		opt.PositiveIntValue = proto.Uint64(val)
	case float64:
		opt.DoubleValue = proto.Float64(val)
	case string:
		opt.StringValue = []byte(val)
	case identifier:
		opt.IdentifierValue = proto.String(string(val))
	case []*aggregateEntryNode:
		var buf bytes.Buffer
		aggToString(val, &buf)
		aggStr := buf.String()
		opt.AggregateValue = proto.String(aggStr)
	}
	return opt
}

func (r *parseResult) asUninterpretedOptionName(parts []*optionNamePartNode) []*dpb.UninterpretedOption_NamePart {
	ret := make([]*dpb.UninterpretedOption_NamePart, len(parts))
	for i, part := range parts {
		txt := part.text.val
		if !part.isExtension {
			txt = part.text.val[part.offset : part.offset+part.length]
		}
		np := &dpb.UninterpretedOption_NamePart{
			NamePart:    proto.String(txt),
			IsExtension: proto.Bool(part.isExtension),
		}
		r.putOptionNamePartNode(np, part)
		ret[i] = np
	}
	return ret
}

func (r *parseResult) addExtensions(ext *extendNode, flds *[]*dpb.FieldDescriptorProto, msgs *[]*dpb.DescriptorProto, isProto3 bool) {
	extendee := ext.extendee.val
	count := 0
	for _, decl := range ext.decls {
		if decl.field != nil {
			count++
			decl.field.extendee = ext
			// use higher limit since we don't know yet whether extendee is messageset wire format
			fd := r.asFieldDescriptor(decl.field, internal.MaxTag, isProto3)
			fd.Extendee = proto.String(extendee)
			*flds = append(*flds, fd)
		} else if decl.group != nil {
			count++
			decl.group.extendee = ext
			// ditto: use higher limit right now
			fd, md := r.asGroupDescriptors(decl.group, isProto3, internal.MaxTag)
			fd.Extendee = proto.String(extendee)
			*flds = append(*flds, fd)
			*msgs = append(*msgs, md)
		}
	}
	if count == 0 {
		_ = r.errs.handleErrorWithPos(ext.start(), "extend sections must define at least one extension")
	}
}

func asLabel(lbl *fieldLabel) *dpb.FieldDescriptorProto_Label {
	if lbl.identNode == nil {
		return nil
	}
	switch {
	case lbl.repeated:
		return dpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
	case lbl.required:
		return dpb.FieldDescriptorProto_LABEL_REQUIRED.Enum()
	default:
		return dpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum()
	}
}

func (r *parseResult) asFieldDescriptor(node *fieldNode, maxTag int32, isProto3 bool) *dpb.FieldDescriptorProto {
	tag := node.tag.val
	if err := checkTag(node.tag.start(), tag, maxTag); err != nil {
		_ = r.errs.handleError(err)
	}
	fd := newFieldDescriptor(node.name.val, node.fldType.val, int32(tag), asLabel(&node.label))
	r.putFieldNode(fd, node)
	if opts := node.options.Elements(); len(opts) > 0 {
		fd.Options = &dpb.FieldOptions{UninterpretedOption: r.asUninterpretedOptions(opts)}
	}
	if isProto3 && fd.Label != nil && fd.GetLabel() == dpb.FieldDescriptorProto_LABEL_OPTIONAL {
		internal.SetProto3Optional(fd)
	}
	return fd
}

var fieldTypes = map[string]dpb.FieldDescriptorProto_Type{
	"double":   dpb.FieldDescriptorProto_TYPE_DOUBLE,
	"float":    dpb.FieldDescriptorProto_TYPE_FLOAT,
	"int32":    dpb.FieldDescriptorProto_TYPE_INT32,
	"int64":    dpb.FieldDescriptorProto_TYPE_INT64,
	"uint32":   dpb.FieldDescriptorProto_TYPE_UINT32,
	"uint64":   dpb.FieldDescriptorProto_TYPE_UINT64,
	"sint32":   dpb.FieldDescriptorProto_TYPE_SINT32,
	"sint64":   dpb.FieldDescriptorProto_TYPE_SINT64,
	"fixed32":  dpb.FieldDescriptorProto_TYPE_FIXED32,
	"fixed64":  dpb.FieldDescriptorProto_TYPE_FIXED64,
	"sfixed32": dpb.FieldDescriptorProto_TYPE_SFIXED32,
	"sfixed64": dpb.FieldDescriptorProto_TYPE_SFIXED64,
	"bool":     dpb.FieldDescriptorProto_TYPE_BOOL,
	"string":   dpb.FieldDescriptorProto_TYPE_STRING,
	"bytes":    dpb.FieldDescriptorProto_TYPE_BYTES,
}

func newFieldDescriptor(name string, fieldType string, tag int32, lbl *dpb.FieldDescriptorProto_Label) *dpb.FieldDescriptorProto {
	fd := &dpb.FieldDescriptorProto{
		Name:     proto.String(name),
		JsonName: proto.String(internal.JsonName(name)),
		Number:   proto.Int32(tag),
		Label:    lbl,
	}
	t, ok := fieldTypes[fieldType]
	if ok {
		fd.Type = t.Enum()
	} else {
		// NB: we don't have enough info to determine whether this is an enum
		// or a message type, so we'll leave Type nil and set it later
		// (during linking)
		fd.TypeName = proto.String(fieldType)
	}
	return fd
}

func (r *parseResult) asGroupDescriptors(group *groupNode, isProto3 bool, maxTag int32) (*dpb.FieldDescriptorProto, *dpb.DescriptorProto) {
	tag := group.tag.val
	if err := checkTag(group.tag.start(), tag, maxTag); err != nil {
		_ = r.errs.handleError(err)
	}
	if !unicode.IsUpper(rune(group.name.val[0])) {
		_ = r.errs.handleErrorWithPos(group.name.start(), "group %s should have a name that starts with a capital letter", group.name.val)
	}
	fieldName := strings.ToLower(group.name.val)
	fd := &dpb.FieldDescriptorProto{
		Name:     proto.String(fieldName),
		JsonName: proto.String(internal.JsonName(fieldName)),
		Number:   proto.Int32(int32(tag)),
		Label:    asLabel(&group.label),
		Type:     dpb.FieldDescriptorProto_TYPE_GROUP.Enum(),
		TypeName: proto.String(group.name.val),
	}
	r.putFieldNode(fd, group)
	if opts := group.options.Elements(); len(opts) > 0 {
		fd.Options = &dpb.FieldOptions{UninterpretedOption: r.asUninterpretedOptions(opts)}
	}
	md := &dpb.DescriptorProto{Name: proto.String(group.name.val)}
	r.putMessageNode(md, group)
	r.addMessageDecls(md, group.decls, isProto3)
	return fd, md
}

func (r *parseResult) asMapDescriptors(mapField *mapFieldNode, isProto3 bool, maxTag int32) (*dpb.FieldDescriptorProto, *dpb.DescriptorProto) {
	tag := mapField.tag.val
	if err := checkTag(mapField.tag.start(), tag, maxTag); err != nil {
		_ = r.errs.handleError(err)
	}
	var lbl *dpb.FieldDescriptorProto_Label
	if !isProto3 {
		lbl = dpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum()
	}
	keyFd := newFieldDescriptor("key", mapField.mapType.keyType.val, 1, lbl)
	r.putFieldNode(keyFd, mapField.keyField())
	valFd := newFieldDescriptor("value", mapField.mapType.valueType.val, 2, lbl)
	r.putFieldNode(valFd, mapField.valueField())
	entryName := internal.InitCap(internal.JsonName(mapField.name.val)) + "Entry"
	fd := newFieldDescriptor(mapField.name.val, entryName, int32(tag), dpb.FieldDescriptorProto_LABEL_REPEATED.Enum())
	if opts := mapField.options.Elements(); len(opts) > 0 {
		fd.Options = &dpb.FieldOptions{UninterpretedOption: r.asUninterpretedOptions(opts)}
	}
	r.putFieldNode(fd, mapField)
	md := &dpb.DescriptorProto{
		Name:    proto.String(entryName),
		Options: &dpb.MessageOptions{MapEntry: proto.Bool(true)},
		Field:   []*dpb.FieldDescriptorProto{keyFd, valFd},
	}
	r.putMessageNode(md, mapField)
	return fd, md
}

func (r *parseResult) asExtensionRanges(node *extensionRangeNode, maxTag int32) []*dpb.DescriptorProto_ExtensionRange {
	opts := r.asUninterpretedOptions(node.options.Elements())
	ers := make([]*dpb.DescriptorProto_ExtensionRange, len(node.ranges))
	for i, rng := range node.ranges {
		start, end := getRangeBounds(r, rng, 0, maxTag)
		er := &dpb.DescriptorProto_ExtensionRange{
			Start: proto.Int32(start),
			End:   proto.Int32(end + 1),
		}
		if len(opts) > 0 {
			er.Options = &dpb.ExtensionRangeOptions{UninterpretedOption: opts}
		}
		r.putExtensionRangeNode(er, rng)
		ers[i] = er
	}
	return ers
}

func (r *parseResult) asEnumValue(ev *enumValueNode) *dpb.EnumValueDescriptorProto {
	num, ok := ev.number.asInt32(math.MinInt32, math.MaxInt32)
	if !ok {
		_ = r.errs.handleErrorWithPos(ev.number.start(), "value %d is out of range: should be between %d and %d", ev.number.value(), math.MinInt32, math.MaxInt32)
	}
	evd := &dpb.EnumValueDescriptorProto{Name: proto.String(ev.name.val), Number: proto.Int32(num)}
	r.putEnumValueNode(evd, ev)
	if opts := ev.options.Elements(); len(opts) > 0 {
		evd.Options = &dpb.EnumValueOptions{UninterpretedOption: r.asUninterpretedOptions(opts)}
	}
	return evd
}

func (r *parseResult) asMethodDescriptor(node *methodNode) *dpb.MethodDescriptorProto {
	md := &dpb.MethodDescriptorProto{
		Name:       proto.String(node.name.val),
		InputType:  proto.String(node.input.msgType.val),
		OutputType: proto.String(node.output.msgType.val),
	}
	r.putMethodNode(md, node)
	if node.input.streamKeyword != nil {
		md.ClientStreaming = proto.Bool(true)
	}
	if node.output.streamKeyword != nil {
		md.ServerStreaming = proto.Bool(true)
	}
	// protoc always adds a MethodOptions if there are brackets
	// We have a non-nil node.options if there are brackets
	// We do the same to match protoc as closely as possible
	// https://github.com/protocolbuffers/protobuf/blob/0c3f43a6190b77f1f68b7425d1b7e1a8257a8d0c/src/google/protobuf/compiler/parser.cc#L2152
	if node.options != nil {
		md.Options = &dpb.MethodOptions{UninterpretedOption: r.asUninterpretedOptions(node.options)}
	}
	return md
}

func (r *parseResult) asEnumDescriptor(en *enumNode) *dpb.EnumDescriptorProto {
	ed := &dpb.EnumDescriptorProto{Name: proto.String(en.name.val)}
	r.putEnumNode(ed, en)
	for _, decl := range en.decls {
		if decl.option != nil {
			if ed.Options == nil {
				ed.Options = &dpb.EnumOptions{}
			}
			ed.Options.UninterpretedOption = append(ed.Options.UninterpretedOption, r.asUninterpretedOption(decl.option))
		} else if decl.value != nil {
			ed.Value = append(ed.Value, r.asEnumValue(decl.value))
		} else if decl.reserved != nil {
			for _, n := range decl.reserved.names {
				ed.ReservedName = append(ed.ReservedName, n.val)
			}
			for _, rng := range decl.reserved.ranges {
				ed.ReservedRange = append(ed.ReservedRange, r.asEnumReservedRange(rng))
			}
		}
	}
	return ed
}

func (r *parseResult) asEnumReservedRange(rng *rangeNode) *dpb.EnumDescriptorProto_EnumReservedRange {
	start, end := getRangeBounds(r, rng, math.MinInt32, math.MaxInt32)
	rr := &dpb.EnumDescriptorProto_EnumReservedRange{
		Start: proto.Int32(start),
		End:   proto.Int32(end),
	}
	r.putEnumReservedRangeNode(rr, rng)
	return rr
}

func (r *parseResult) asMessageDescriptor(node *messageNode, isProto3 bool) *dpb.DescriptorProto {
	msgd := &dpb.DescriptorProto{Name: proto.String(node.name.val)}
	r.putMessageNode(msgd, node)
	r.addMessageDecls(msgd, node.decls, isProto3)
	return msgd
}

func (r *parseResult) addMessageDecls(msgd *dpb.DescriptorProto, decls []*messageElement, isProto3 bool) {
	// first process any options
	for _, decl := range decls {
		if decl.option != nil {
			if msgd.Options == nil {
				msgd.Options = &dpb.MessageOptions{}
			}
			msgd.Options.UninterpretedOption = append(msgd.Options.UninterpretedOption, r.asUninterpretedOption(decl.option))
		}
	}

	// now that we have options, we can see if this uses messageset wire format, which
	// impacts how we validate tag numbers in any fields in the message
	maxTag := int32(internal.MaxNormalTag)
	if isMessageSet, err := isMessageSetWireFormat(r, "message "+msgd.GetName(), msgd); err != nil {
		return
	} else if isMessageSet {
		maxTag = internal.MaxTag // higher limit for messageset wire format
	}

	rsvdNames := map[string]int{}

	// now we can process the rest
	for _, decl := range decls {
		if decl.enum != nil {
			msgd.EnumType = append(msgd.EnumType, r.asEnumDescriptor(decl.enum))
		} else if decl.extend != nil {
			r.addExtensions(decl.extend, &msgd.Extension, &msgd.NestedType, isProto3)
		} else if decl.extensionRange != nil {
			msgd.ExtensionRange = append(msgd.ExtensionRange, r.asExtensionRanges(decl.extensionRange, maxTag)...)
		} else if decl.field != nil {
			fd := r.asFieldDescriptor(decl.field, maxTag, isProto3)
			msgd.Field = append(msgd.Field, fd)
		} else if decl.mapField != nil {
			fd, md := r.asMapDescriptors(decl.mapField, isProto3, maxTag)
			msgd.Field = append(msgd.Field, fd)
			msgd.NestedType = append(msgd.NestedType, md)
		} else if decl.group != nil {
			fd, md := r.asGroupDescriptors(decl.group, isProto3, maxTag)
			msgd.Field = append(msgd.Field, fd)
			msgd.NestedType = append(msgd.NestedType, md)
		} else if decl.oneOf != nil {
			oodIndex := len(msgd.OneofDecl)
			ood := &dpb.OneofDescriptorProto{Name: proto.String(decl.oneOf.name.val)}
			r.putOneOfNode(ood, decl.oneOf)
			msgd.OneofDecl = append(msgd.OneofDecl, ood)
			ooFields := 0
			for _, oodecl := range decl.oneOf.decls {
				if oodecl.option != nil {
					if ood.Options == nil {
						ood.Options = &dpb.OneofOptions{}
					}
					ood.Options.UninterpretedOption = append(ood.Options.UninterpretedOption, r.asUninterpretedOption(oodecl.option))
				} else if oodecl.field != nil {
					fd := r.asFieldDescriptor(oodecl.field, maxTag, isProto3)
					fd.OneofIndex = proto.Int32(int32(oodIndex))
					msgd.Field = append(msgd.Field, fd)
					ooFields++
				} else if oodecl.group != nil {
					fd, md := r.asGroupDescriptors(oodecl.group, isProto3, maxTag)
					fd.OneofIndex = proto.Int32(int32(oodIndex))
					msgd.Field = append(msgd.Field, fd)
					msgd.NestedType = append(msgd.NestedType, md)
					ooFields++
				}
			}
			if ooFields == 0 {
				_ = r.errs.handleErrorWithPos(decl.oneOf.start(), "oneof must contain at least one field")
			}
		} else if decl.nested != nil {
			msgd.NestedType = append(msgd.NestedType, r.asMessageDescriptor(decl.nested, isProto3))
		} else if decl.reserved != nil {
			for _, n := range decl.reserved.names {
				count := rsvdNames[n.val]
				if count == 1 { // already seen
					_ = r.errs.handleErrorWithPos(n.start(), "name %q is reserved multiple times", n.val)
				}
				rsvdNames[n.val] = count + 1
				msgd.ReservedName = append(msgd.ReservedName, n.val)
			}
			for _, rng := range decl.reserved.ranges {
				msgd.ReservedRange = append(msgd.ReservedRange, r.asMessageReservedRange(rng, maxTag))
			}
		}
	}

	// process any proto3_optional fields
	if isProto3 {
		internal.ProcessProto3OptionalFields(msgd)
	}
}

func isMessageSetWireFormat(res *parseResult, scope string, md *dpb.DescriptorProto) (bool, error) {
	uo := md.GetOptions().GetUninterpretedOption()
	index, err := findOption(res, scope, uo, "message_set_wire_format")
	if err != nil {
		return false, err
	}
	if index == -1 {
		// no such option, so default to false
		return false, nil
	}

	opt := uo[index]
	optNode := res.getOptionNode(opt)

	switch opt.GetIdentifierValue() {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, res.errs.handleErrorWithPos(optNode.getValue().start(), "%s: expecting bool value for message_set_wire_format option", scope)
	}
}

func (r *parseResult) asMessageReservedRange(rng *rangeNode, maxTag int32) *dpb.DescriptorProto_ReservedRange {
	start, end := getRangeBounds(r, rng, 0, maxTag)
	rr := &dpb.DescriptorProto_ReservedRange{
		Start: proto.Int32(start),
		End:   proto.Int32(end + 1),
	}
	r.putMessageReservedRangeNode(rr, rng)
	return rr
}

func getRangeBounds(res *parseResult, rng *rangeNode, minVal, maxVal int32) (int32, int32) {
	checkOrder := true
	start, ok := rng.startValueAsInt32(minVal, maxVal)
	if !ok {
		checkOrder = false
		_ = res.errs.handleErrorWithPos(rng.startNode.start(), "range start %d is out of range: should be between %d and %d", rng.startValue(), minVal, maxVal)
	}

	end, ok := rng.endValueAsInt32(minVal, maxVal)
	if !ok {
		checkOrder = false
		if rng.endNode != nil {
			_ = res.errs.handleErrorWithPos(rng.endNode.start(), "range end %d is out of range: should be between %d and %d", rng.endValue(), minVal, maxVal)
		}
	}

	if checkOrder && start > end {
		_ = res.errs.handleErrorWithPos(rng.rangeStart().start(), "range, %d to %d, is invalid: start must be <= end", start, end)
	}

	return start, end
}

func (r *parseResult) asServiceDescriptor(svc *serviceNode) *dpb.ServiceDescriptorProto {
	sd := &dpb.ServiceDescriptorProto{Name: proto.String(svc.name.val)}
	r.putServiceNode(sd, svc)
	for _, decl := range svc.decls {
		if decl.option != nil {
			if sd.Options == nil {
				sd.Options = &dpb.ServiceOptions{}
			}
			sd.Options.UninterpretedOption = append(sd.Options.UninterpretedOption, r.asUninterpretedOption(decl.option))
		} else if decl.rpc != nil {
			sd.Method = append(sd.Method, r.asMethodDescriptor(decl.rpc))
		}
	}
	return sd
}
