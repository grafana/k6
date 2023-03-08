package common

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type bridgeTestFieldsType struct {
	Exported       string
	ExportedTag    string `js:"renamed"`
	ExportedHidden string `js:"-"`
	unexported     string //nolint:structcheck,unused // actually checked in the test
	unexportedTag  string `js:"unexported"` //nolint:structcheck,unused // actually checked in the test
}

type bridgeTestMethodsType struct{}

func (bridgeTestMethodsType) ExportedFn() {}

//nolint:unused // needed for the actual test to check that it won't be seen
func (bridgeTestMethodsType) unexportedFn() {}

func (*bridgeTestMethodsType) ExportedPtrFn() {}

//nolint:unused // needed for the actual test to check that it won't be seen
func (*bridgeTestMethodsType) unexportedPtrFn() {}

type bridgeTestOddFieldsType struct {
	TwoWords string
	URL      string
}

type bridgeTestConstructorType struct{}

type bridgeTestConstructorSpawnedType struct{}

func (bridgeTestConstructorType) XConstructor() bridgeTestConstructorSpawnedType {
	return bridgeTestConstructorSpawnedType{}
}

func TestFieldNameMapper(t *testing.T) {
	t.Parallel()
	testdata := []struct {
		Typ     reflect.Type
		Fields  map[string]string
		Methods map[string]string
	}{
		{reflect.TypeOf(bridgeTestFieldsType{}), map[string]string{
			"Exported":       "exported",
			"ExportedTag":    "renamed",
			"ExportedHidden": "",
			"unexported":     "",
			"unexportedTag":  "",
		}, nil},
		{reflect.TypeOf(bridgeTestMethodsType{}), nil, map[string]string{
			"ExportedFn":   "exportedFn",
			"unexportedFn": "",
		}},
		{reflect.TypeOf(bridgeTestOddFieldsType{}), map[string]string{
			"TwoWords": "two_words",
			"URL":      "url",
		}, nil},
		{reflect.TypeOf(bridgeTestConstructorType{}), nil, map[string]string{
			"XConstructor": "Constructor",
		}},
	}
	for _, data := range testdata {
		data := data
		for field, name := range data.Fields {
			field, name := field, name
			t.Run(field, func(t *testing.T) {
				t.Parallel()
				f, ok := data.Typ.FieldByName(field)
				require.True(t, ok, "no such field")
				assert.Equal(t, name, (FieldNameMapper{}).FieldName(data.Typ, f))
			})
		}
		for meth, name := range data.Methods {
			meth, name := meth, name
			t.Run(meth, func(t *testing.T) {
				t.Parallel()
				m, ok := data.Typ.MethodByName(meth)
				if name != "" {
					require.True(t, ok, "no such method")
					assert.Equal(t, name, (FieldNameMapper{}).MethodName(data.Typ, m))
				} else {
					assert.False(t, ok, "exported by accident")
				}
			})
		}
	}
}
