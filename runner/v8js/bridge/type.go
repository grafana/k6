package bridge

import (
	"errors"
	"reflect"
)

type Type struct {
	Kind    reflect.Kind
	Spec    map[string]Type
	JSONKey string
}

// Creates a bridged type.
// Panics if raw is a function.
func BridgeType(raw reflect.Type) Type {
	tp := Type{Kind: raw.Kind()}

	if tp.Kind == reflect.Func {
		panic(errors.New("That's a function, bridge it as such"))
	}

	if tp.Kind == reflect.Struct {
		tp.Spec = make(map[string]Type)
		for i := 0; i < raw.NumField(); i++ {
			f := raw.Field(i)
			if f.Anonymous {
				continue
			}

			ftp := BridgeType(f.Type)

			ftp.JSONKey = f.Name
			if tag := f.Tag.Get("json"); tag != "" {
				ftp.JSONKey = tag
			}
			tp.Spec[f.Name] = ftp
		}
	}

	return tp
}
