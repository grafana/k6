package protoparse

import (
	"fmt"
	"sort"

	"github.com/golang/protobuf/proto"

	dpb "github.com/golang/protobuf/protoc-gen-go/descriptor"
)

func validateBasic(res *parseResult, containsErrors bool) {
	fd := res.fd
	isProto3 := fd.GetSyntax() == "proto3"

	for _, md := range fd.MessageType {
		if validateMessage(res, isProto3, "", md, containsErrors) != nil {
			return
		}
	}

	for _, ed := range fd.EnumType {
		if validateEnum(res, isProto3, "", ed, containsErrors) != nil {
			return
		}
	}

	for _, fld := range fd.Extension {
		if validateField(res, isProto3, "", fld) != nil {
			return
		}
	}
}

func validateMessage(res *parseResult, isProto3 bool, prefix string, md *dpb.DescriptorProto, containsErrors bool) error {
	nextPrefix := md.GetName() + "."

	for _, fld := range md.Field {
		if err := validateField(res, isProto3, nextPrefix, fld); err != nil {
			return err
		}
	}
	for _, fld := range md.Extension {
		if err := validateField(res, isProto3, nextPrefix, fld); err != nil {
			return err
		}
	}
	for _, ed := range md.EnumType {
		if err := validateEnum(res, isProto3, nextPrefix, ed, containsErrors); err != nil {
			return err
		}
	}
	for _, nmd := range md.NestedType {
		if err := validateMessage(res, isProto3, nextPrefix, nmd, containsErrors); err != nil {
			return err
		}
	}

	scope := fmt.Sprintf("message %s%s", prefix, md.GetName())

	if isProto3 && len(md.ExtensionRange) > 0 {
		n := res.getExtensionRangeNode(md.ExtensionRange[0])
		if err := res.errs.handleErrorWithPos(n.start(), "%s: extension ranges are not allowed in proto3", scope); err != nil {
			return err
		}
	}

	if index, err := findOption(res, scope, md.Options.GetUninterpretedOption(), "map_entry"); err != nil {
		return err
	} else if index >= 0 {
		opt := md.Options.UninterpretedOption[index]
		optn := res.getOptionNode(opt)
		md.Options.UninterpretedOption = removeOption(md.Options.UninterpretedOption, index)
		valid := false
		if opt.IdentifierValue != nil {
			if opt.GetIdentifierValue() == "true" {
				valid = true
				if err := res.errs.handleErrorWithPos(optn.getValue().start(), "%s: map_entry option should not be set explicitly; use map type instead", scope); err != nil {
					return err
				}
			} else if opt.GetIdentifierValue() == "false" {
				valid = true
				md.Options.MapEntry = proto.Bool(false)
			}
		}
		if !valid {
			if err := res.errs.handleErrorWithPos(optn.getValue().start(), "%s: expecting bool value for map_entry option", scope); err != nil {
				return err
			}
		}
	}

	// reserved ranges should not overlap
	rsvd := make(tagRanges, len(md.ReservedRange))
	for i, r := range md.ReservedRange {
		n := res.getMessageReservedRangeNode(r)
		rsvd[i] = tagRange{start: r.GetStart(), end: r.GetEnd(), node: n}

	}
	sort.Sort(rsvd)
	for i := 1; i < len(rsvd); i++ {
		if rsvd[i].start < rsvd[i-1].end {
			if err := res.errs.handleErrorWithPos(rsvd[i].node.start(), "%s: reserved ranges overlap: %d to %d and %d to %d", scope, rsvd[i-1].start, rsvd[i-1].end-1, rsvd[i].start, rsvd[i].end-1); err != nil {
				return err
			}
		}
	}

	// extensions ranges should not overlap
	exts := make(tagRanges, len(md.ExtensionRange))
	for i, r := range md.ExtensionRange {
		n := res.getExtensionRangeNode(r)
		exts[i] = tagRange{start: r.GetStart(), end: r.GetEnd(), node: n}
	}
	sort.Sort(exts)
	for i := 1; i < len(exts); i++ {
		if exts[i].start < exts[i-1].end {
			if err := res.errs.handleErrorWithPos(exts[i].node.start(), "%s: extension ranges overlap: %d to %d and %d to %d", scope, exts[i-1].start, exts[i-1].end-1, exts[i].start, exts[i].end-1); err != nil {
				return err
			}
		}
	}

	// see if any extension range overlaps any reserved range
	var i, j int // i indexes rsvd; j indexes exts
	for i < len(rsvd) && j < len(exts) {
		if rsvd[i].start >= exts[j].start && rsvd[i].start < exts[j].end ||
			exts[j].start >= rsvd[i].start && exts[j].start < rsvd[i].end {

			var pos *SourcePos
			if rsvd[i].start >= exts[j].start && rsvd[i].start < exts[j].end {
				pos = rsvd[i].node.start()
			} else {
				pos = exts[j].node.start()
			}
			// ranges overlap
			if err := res.errs.handleErrorWithPos(pos, "%s: extension range %d to %d overlaps reserved range %d to %d", scope, exts[j].start, exts[j].end-1, rsvd[i].start, rsvd[i].end-1); err != nil {
				return err
			}
		}
		if rsvd[i].start < exts[j].start {
			i++
		} else {
			j++
		}
	}

	// now, check that fields don't re-use tags and don't try to use extension
	// or reserved ranges or reserved names
	rsvdNames := map[string]struct{}{}
	for _, n := range md.ReservedName {
		rsvdNames[n] = struct{}{}
	}
	fieldTags := map[int32]string{}
	for _, fld := range md.Field {
		fn := res.getFieldNode(fld)
		if _, ok := rsvdNames[fld.GetName()]; ok {
			if err := res.errs.handleErrorWithPos(fn.fieldName().start(), "%s: field %s is using a reserved name", scope, fld.GetName()); err != nil {
				return err
			}
		}
		if existing := fieldTags[fld.GetNumber()]; existing != "" {
			if err := res.errs.handleErrorWithPos(fn.fieldTag().start(), "%s: fields %s and %s both have the same tag %d", scope, existing, fld.GetName(), fld.GetNumber()); err != nil {
				return err
			}
		}
		fieldTags[fld.GetNumber()] = fld.GetName()
		// check reserved ranges
		r := sort.Search(len(rsvd), func(index int) bool { return rsvd[index].end > fld.GetNumber() })
		if r < len(rsvd) && rsvd[r].start <= fld.GetNumber() {
			if err := res.errs.handleErrorWithPos(fn.fieldTag().start(), "%s: field %s is using tag %d which is in reserved range %d to %d", scope, fld.GetName(), fld.GetNumber(), rsvd[r].start, rsvd[r].end-1); err != nil {
				return err
			}
		}
		// and check extension ranges
		e := sort.Search(len(exts), func(index int) bool { return exts[index].end > fld.GetNumber() })
		if e < len(exts) && exts[e].start <= fld.GetNumber() {
			if err := res.errs.handleErrorWithPos(fn.fieldTag().start(), "%s: field %s is using tag %d which is in extension range %d to %d", scope, fld.GetName(), fld.GetNumber(), exts[e].start, exts[e].end-1); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateEnum(res *parseResult, isProto3 bool, prefix string, ed *dpb.EnumDescriptorProto, containsErrors bool) error {
	scope := fmt.Sprintf("enum %s%s", prefix, ed.GetName())

	if !containsErrors && len(ed.Value) == 0 {
		// we only check this if file parsing had no errors; otherwise, the file may have
		// had an enum value, but the parser encountered an error processing it, in which
		// case the value would be absent from the descriptor. In such a case, this error
		// would be confusing and incorrect, so we just skip this check.
		enNode := res.getEnumNode(ed)
		if err := res.errs.handleErrorWithPos(enNode.start(), "%s: enums must define at least one value", scope); err != nil {
			return err
		}
	}

	allowAlias := false
	if index, err := findOption(res, scope, ed.Options.GetUninterpretedOption(), "allow_alias"); err != nil {
		return err
	} else if index >= 0 {
		opt := ed.Options.UninterpretedOption[index]
		valid := false
		if opt.IdentifierValue != nil {
			if opt.GetIdentifierValue() == "true" {
				allowAlias = true
				valid = true
			} else if opt.GetIdentifierValue() == "false" {
				valid = true
			}
		}
		if !valid {
			optNode := res.getOptionNode(opt)
			if err := res.errs.handleErrorWithPos(optNode.getValue().start(), "%s: expecting bool value for allow_alias option", scope); err != nil {
				return err
			}
		}
	}

	if isProto3 && len(ed.Value) > 0 && ed.Value[0].GetNumber() != 0 {
		evNode := res.getEnumValueNode(ed.Value[0])
		if err := res.errs.handleErrorWithPos(evNode.getNumber().start(), "%s: proto3 requires that first value in enum have numeric value of 0", scope); err != nil {
			return err
		}
	}

	if !allowAlias {
		// make sure all value numbers are distinct
		vals := map[int32]string{}
		for _, evd := range ed.Value {
			if existing := vals[evd.GetNumber()]; existing != "" {
				evNode := res.getEnumValueNode(evd)
				if err := res.errs.handleErrorWithPos(evNode.getNumber().start(), "%s: values %s and %s both have the same numeric value %d; use allow_alias option if intentional", scope, existing, evd.GetName(), evd.GetNumber()); err != nil {
					return err
				}
			}
			vals[evd.GetNumber()] = evd.GetName()
		}
	}

	// reserved ranges should not overlap
	rsvd := make(tagRanges, len(ed.ReservedRange))
	for i, r := range ed.ReservedRange {
		n := res.getEnumReservedRangeNode(r)
		rsvd[i] = tagRange{start: r.GetStart(), end: r.GetEnd(), node: n}
	}
	sort.Sort(rsvd)
	for i := 1; i < len(rsvd); i++ {
		if rsvd[i].start <= rsvd[i-1].end {
			if err := res.errs.handleErrorWithPos(rsvd[i].node.start(), "%s: reserved ranges overlap: %d to %d and %d to %d", scope, rsvd[i-1].start, rsvd[i-1].end, rsvd[i].start, rsvd[i].end); err != nil {
				return err
			}
		}
	}

	// now, check that fields don't re-use tags and don't try to use extension
	// or reserved ranges or reserved names
	rsvdNames := map[string]struct{}{}
	for _, n := range ed.ReservedName {
		rsvdNames[n] = struct{}{}
	}
	for _, ev := range ed.Value {
		evn := res.getEnumValueNode(ev)
		if _, ok := rsvdNames[ev.GetName()]; ok {
			if err := res.errs.handleErrorWithPos(evn.getName().start(), "%s: value %s is using a reserved name", scope, ev.GetName()); err != nil {
				return err
			}
		}
		// check reserved ranges
		r := sort.Search(len(rsvd), func(index int) bool { return rsvd[index].end >= ev.GetNumber() })
		if r < len(rsvd) && rsvd[r].start <= ev.GetNumber() {
			if err := res.errs.handleErrorWithPos(evn.getNumber().start(), "%s: value %s is using number %d which is in reserved range %d to %d", scope, ev.GetName(), ev.GetNumber(), rsvd[r].start, rsvd[r].end); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateField(res *parseResult, isProto3 bool, prefix string, fld *dpb.FieldDescriptorProto) error {
	scope := fmt.Sprintf("field %s%s", prefix, fld.GetName())

	node := res.getFieldNode(fld)
	if isProto3 {
		if fld.GetType() == dpb.FieldDescriptorProto_TYPE_GROUP {
			n := node.(*groupNode)
			if err := res.errs.handleErrorWithPos(n.groupKeyword.start(), "%s: groups are not allowed in proto3", scope); err != nil {
				return err
			}
		} else if fld.Label != nil && fld.GetLabel() == dpb.FieldDescriptorProto_LABEL_REQUIRED {
			if err := res.errs.handleErrorWithPos(node.fieldLabel().start(), "%s: label 'required' is not allowed in proto3", scope); err != nil {
				return err
			}
		} else if fld.Extendee != nil && fld.Label != nil && fld.GetLabel() == dpb.FieldDescriptorProto_LABEL_OPTIONAL {
			if err := res.errs.handleErrorWithPos(node.fieldLabel().start(), "%s: label 'optional' is not allowed on extensions in proto3", scope); err != nil {
				return err
			}
		}
		if index, err := findOption(res, scope, fld.Options.GetUninterpretedOption(), "default"); err != nil {
			return err
		} else if index >= 0 {
			optNode := res.getOptionNode(fld.Options.GetUninterpretedOption()[index])
			if err := res.errs.handleErrorWithPos(optNode.getName().start(), "%s: default values are not allowed in proto3", scope); err != nil {
				return err
			}
		}
	} else {
		if fld.Label == nil && fld.OneofIndex == nil {
			if err := res.errs.handleErrorWithPos(node.fieldName().start(), "%s: field has no label; proto2 requires explicit 'optional' label", scope); err != nil {
				return err
			}
		}
		if fld.GetExtendee() != "" && fld.Label != nil && fld.GetLabel() == dpb.FieldDescriptorProto_LABEL_REQUIRED {
			if err := res.errs.handleErrorWithPos(node.fieldLabel().start(), "%s: extension fields cannot be 'required'", scope); err != nil {
				return err
			}
		}
	}

	// finally, set any missing label to optional
	if fld.Label == nil {
		fld.Label = dpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum()
	}

	return nil
}

type tagRange struct {
	start int32
	end   int32
	node  rangeDecl
}

type tagRanges []tagRange

func (r tagRanges) Len() int {
	return len(r)
}

func (r tagRanges) Less(i, j int) bool {
	return r[i].start < r[j].start ||
		(r[i].start == r[j].start && r[i].end < r[j].end)
}

func (r tagRanges) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}
