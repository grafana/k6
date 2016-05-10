package bridge

import (
	"errors"
	log "github.com/Sirupsen/logrus"
	"reflect"
)

type Type struct {
	Type    reflect.Type
	Spec    map[string]Type
	JSONKey string
}

func (t *Type) Cast(v *interface{}) error {
	rV := reflect.ValueOf(*v)
	vT := rV.Type()
	if vT == t.Type {
		return nil
	}

	switch t.Type.Kind() {
	case reflect.Struct:
		if vT.Kind() != reflect.Map {
			return errors.New("Invalid argument")
		}
	default:
		if !vT.ConvertibleTo(t.Type) {
			log.WithFields(log.Fields{
				"expected": t.Type,
				"actual":   vT,
			}).Debug("Invalid argument")
			return errors.New("Invalid argument")
		}
		rV = rV.Convert(t.Type)
		*v = rV.Interface()
	}

	return nil
}

// Creates a bridged type.
// Panics if raw is a function.
func BridgeType(raw reflect.Type) Type {
	tp := Type{Type: raw}
	kind := tp.Type.Kind()

	if kind == reflect.Func {
		panic(errors.New("That's a function, bridge it as such"))
	}

	if kind == reflect.Struct {
		tp.Spec = make(map[string]Type)
		for i := 0; i < raw.NumField(); i++ {
			f := raw.Field(i)
			tag := f.Tag.Get("json")
			if tag == "" || tag == "-" {
				continue
			}

			ftp := BridgeType(f.Type)
			ftp.JSONKey = tag
			tp.Spec[f.Name] = ftp
		}
	}

	return tp
}
