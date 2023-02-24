package internal

import (
	"reflect"

	"github.com/golang/protobuf/proto"
)

var typeOfBytes = reflect.TypeOf([]byte(nil))

// GetUnrecognized fetches the bytes of unrecognized fields for the given message.
func GetUnrecognized(msg proto.Message) []byte {
	val := reflect.Indirect(reflect.ValueOf(msg))
	u := val.FieldByName("XXX_unrecognized")
	if u.IsValid() && u.Type() == typeOfBytes {
		return u.Interface().([]byte)
	}
	// if we didn't get it from the field, try using V2 API to get it
	return proto.MessageReflect(msg).GetUnknown()
}

// SetUnrecognized adds the given bytes to the unrecognized fields for the given message.
func SetUnrecognized(msg proto.Message, data []byte) {
	val := reflect.Indirect(reflect.ValueOf(msg))
	u := val.FieldByName("XXX_unrecognized")
	if u.IsValid() && u.Type() == typeOfBytes {
		// Just store the bytes in the unrecognized field
		ub := u.Interface().([]byte)
		ub = append(ub, data...)
		u.Set(reflect.ValueOf(ub))
		return
	}

	// if we can't set the field, try using V2 API to get it
	mr := proto.MessageReflect(msg)
	existing := mr.GetUnknown()
	if len(existing) > 0 {
		data = append(existing, data...)
	}
	mr.SetUnknown(data)
}
