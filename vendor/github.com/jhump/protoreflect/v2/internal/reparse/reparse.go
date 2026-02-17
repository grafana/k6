package reparse

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/jhump/protoreflect/v2/internal"
)

// ReparseUnrecognized uses the given resolver to re-parse any unrecognized
// fields in msg. It returns true if any changes were made due to successfully
// re-parsing fields.
func ReparseUnrecognized(msg protoreflect.Message, resolver resolver) bool {
	var changed bool
	msg.Range(func(fld protoreflect.FieldDescriptor, val protoreflect.Value) bool {
		if !internal.IsMessageKind(fld.Kind()) {
			return true
		}
		if fld.IsList() {
			l := val.List()
			for i := 0; i < l.Len(); i++ {
				if ReparseUnrecognized(l.Get(i).Message(), resolver) {
					changed = true
				}
			}
		} else if fld.IsMap() {
			mapVal := fld.MapValue()
			if !internal.IsMessageKind(mapVal.Kind()) {
				return true
			}
			m := val.Map()
			m.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
				if ReparseUnrecognized(v.Message(), resolver) {
					changed = true
				}
				return true
			})
		} else if ReparseUnrecognized(val.Message(), resolver) {
			changed = true
		}
		return true
	})

	unk := msg.GetUnknown()
	if len(unk) > 0 {
		other := msg.New().Interface()
		if err := (proto.UnmarshalOptions{Resolver: resolver}).Unmarshal(unk, other); err == nil {
			msg.SetUnknown(nil)
			proto.Merge(msg.Interface(), other)
			changed = true
		}
	}

	return changed
}

// resolver is the interface required by the Resolver field
// of proto.UnmarshalOptions.
type resolver interface {
	FindExtensionByName(field protoreflect.FullName) (protoreflect.ExtensionType, error)
	FindExtensionByNumber(message protoreflect.FullName, field protoreflect.FieldNumber) (protoreflect.ExtensionType, error)
}
