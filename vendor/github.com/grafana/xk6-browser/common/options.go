package common

import (
	"fmt"
	"reflect"
	"time"

	"github.com/dop251/goja"

	"go.k6.io/k6/lib/types"
)

func parseBoolOpt(key string, val goja.Value) (b bool, err error) {
	if val.ExportType().Kind() != reflect.Bool {
		return false, fmt.Errorf("%s should be a boolean", key)
	}
	b, _ = val.Export().(bool)
	return b, nil
}

func parseStrOpt(key string, val goja.Value) (s string, err error) {
	if val.ExportType().Kind() != reflect.String {
		return "", fmt.Errorf("%s should be a string", key)
	}
	return val.String(), nil
}

func parseTimeOpt(key string, val goja.Value) (t time.Duration, err error) {
	if t, err = types.GetDurationValue(val.String()); err != nil {
		return time.Duration(0), fmt.Errorf("%s should be a time duration value: %w", key, err)
	}
	return
}

// exportOpt exports src to dst and dynamically returns an error
// depending on the type if an error occurs. Panics if dst is not
// a pointer and not points to a map, struct, or slice.
func exportOpt[T any](rt *goja.Runtime, key string, src goja.Value, dst T) error {
	typ := reflect.TypeOf(dst)
	if typ.Kind() != reflect.Pointer {
		panic("dst should be a pointer")
	}
	kind := typ.Elem().Kind()
	s, ok := map[reflect.Kind]string{
		reflect.Map:    "a map",
		reflect.Struct: "an object",
		reflect.Slice:  "an array of",
	}[kind]
	if !ok {
		panic("dst should be one of: map, struct, slice")
	}
	if err := rt.ExportTo(src, dst); err != nil {
		if kind == reflect.Slice {
			s += fmt.Sprintf(" %ss", typ.Elem().Elem())
		}
		return fmt.Errorf("%s should be %s: %w", key, s, err)
	}

	return nil
}
