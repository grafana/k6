package common

import (
	"reflect"
	"strings"

	"github.com/serenize/snaker"
)

// if a fieldName is the key of this map exactly than the value for the given key should be used as
// the name of the field in js
//
//nolint:gochecknoglobals
var fieldNameExceptions = map[string]string{
	"OCSP": "ocsp",
}

// FieldName Returns the JS name for an exported struct field. The name is snake_cased, with respect for
// certain common initialisms (URL, ID, HTTP, etc).
func FieldName(t reflect.Type, f reflect.StructField) string {
	// PkgPath is non-empty for unexported fields.
	if f.PkgPath != "" {
		return ""
	}

	// Allow a `js:"name"` tag to override the default name.
	if tag := f.Tag.Get("js"); tag != "" {
		// Matching encoding/json, `js:"-"` hides a field.
		if tag == "-" {
			return ""
		}
		return tag
	}

	if exception, ok := fieldNameExceptions[f.Name]; ok {
		return exception
	}

	// Default to lowercasing the first character of the field name.
	return snaker.CamelToSnake(f.Name)
}

// if a methodName is the key of this map exactly than the value for the given key should be used as
// the name of the method in js
//
//nolint:gochecknoglobals
var methodNameExceptions = map[string]string{
	"JSON": "json",
	"HTML": "html",
	"URL":  "url",
	"OCSP": "ocsp",
}

// MethodName Returns the JS name for an exported method. The first letter of the method's name is
// lowercased, otherwise it is unaltered.
func MethodName(t reflect.Type, m reflect.Method) string {
	// A field with a name beginning with an X is a constructor, and just gets the prefix stripped.
	// Note: They also get some special treatment from Bridge(), see further down.
	if m.Name[0] == 'X' {
		return m.Name[1:]
	}

	if exception, ok := methodNameExceptions[m.Name]; ok {
		return exception
	}
	// Lowercase the first character of the method name.
	return strings.ToLower(m.Name[0:1]) + m.Name[1:]
}

// FieldNameMapper for goja.Runtime.SetFieldNameMapper()
type FieldNameMapper struct{}

// FieldName is part of the goja.FieldNameMapper interface
// https://godoc.org/github.com/dop251/goja#FieldNameMapper
func (FieldNameMapper) FieldName(t reflect.Type, f reflect.StructField) string {
	return FieldName(t, f)
}

// MethodName is part of the goja.FieldNameMapper interface
// https://godoc.org/github.com/dop251/goja#FieldNameMapper
func (FieldNameMapper) MethodName(t reflect.Type, m reflect.Method) string { return MethodName(t, m) }
