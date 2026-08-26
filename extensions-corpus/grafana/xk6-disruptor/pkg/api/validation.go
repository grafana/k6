package api

import (
	"fmt"
	"reflect"
)

// IsCompatible checks if the actual value can be assigned to a variable of the expected type
// For Slices it expects []interface{} and therefore cannot check the type of the elements.
// For Maps it expects map[string]interface{} and therefore cannot check the type of the elements
func IsCompatible(actual interface{}, expected interface{}) error {
	actualType := reflect.TypeOf(actual)
	expectedType := reflect.TypeOf(expected)

	var compatible bool
	switch expectedType.Kind() {
	case reflect.Map:
		compatible = actualType.Kind() == reflect.Map
	case reflect.Slice:
		compatible = actualType.Kind() == reflect.Slice
	case reflect.String:
		compatible = actualType.Kind() == reflect.String
	case reflect.Struct:
		return ValidateStruct(actual, expected)
	default:
		compatible = actualType.ConvertibleTo(expectedType)
	}

	if !compatible {
		return fmt.Errorf("expected %s got %s", expectedType, actualType)
	}
	return nil
}

// ValidateStruct validates that the value of a generic map[string]interface{} can
// be assigned to a expected Struct using the compatibility rules defined in IsCompatible.
// Note that the field names are expected to match except for the case of the initial letter
// e.g  'fieldName' will match 'FieldName' in the struct, but 'field_name' will not.
//
// TODO: use the tags in the struct to find out any field name mapping. This will require
// iterating from the struct to the actual value. This will not detect spurious fields in the
// actual value
func ValidateStruct(actual interface{}, expected interface{}) error {
	actualValue, ok := actual.(map[string]interface{})
	if !ok {
		return fmt.Errorf("expected actual value map[string]interface{} got %s", reflect.TypeOf(actual))
	}
	expectedType := reflect.TypeOf(expected)
	if expectedType.Kind() != reflect.Struct {
		return fmt.Errorf("expected value must be a Struct")
	}
	expectedValue := reflect.ValueOf(expected)

	for field, value := range actualValue {
		sf, found := expectedType.FieldByName(toGoCase(field))
		if !found {
			return fmt.Errorf("unknown field %s in struct %s", field, expectedType.Name())
		}

		err := IsCompatible(value, expectedValue.FieldByIndex(sf.Index).Interface())
		if err != nil {
			return fmt.Errorf("invalid type for field %s: %w", field, err)
		}

		if sf.Type.Kind() == reflect.Struct {
			sfv := expectedValue.FieldByName(sf.Name).Interface()
			err := ValidateStruct(value, sfv)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
