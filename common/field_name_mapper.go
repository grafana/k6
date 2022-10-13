package common

import (
	"reflect"

	k6common "go.k6.io/k6/js/common"
)

// FieldNameMapper for goja.Runtime.SetFieldNameMapper().
type FieldNameMapper struct {
	parent k6common.FieldNameMapper
}

var methodNameExceptions = map[string]string{
	"Query":    "$",
	"QueryAll": "$$",
}

// NewFieldNameMapper creates a new field name mapper to add some method name
// exceptions needed for the xk6-browser extension.
func NewFieldNameMapper() *FieldNameMapper {
	return &FieldNameMapper{
		parent: k6common.FieldNameMapper{},
	}
}

// FieldName is part of the goja.FieldNameMapper interface
// https://godoc.org/github.com/dop251/goja#FieldNameMapper
func (fm *FieldNameMapper) FieldName(t reflect.Type, f reflect.StructField) string {
	return fm.parent.FieldName(t, f)
}

// MethodName is part of the goja.FieldNameMapper interface
// https://godoc.org/github.com/dop251/goja#FieldNameMapper
func (fm *FieldNameMapper) MethodName(t reflect.Type, m reflect.Method) string {
	if exception, ok := methodNameExceptions[m.Name]; ok {
		return exception
	}
	return fm.parent.MethodName(t, m)
}
