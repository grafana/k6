package api

import (
	"fmt"
	"math"
	"reflect"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/types/intstr"
)

// Convert converts from a generic object received from the JS interface via goja into a go type.
// It supports the following conversions:
// Target golang value       Value from JS
// struct                <-- map[string]interface{}
// map[string]ValueType  <-- map[string]interface (1)
// []type                <-- []interface{}
// float64               <-- float64
// int64                 <-- int64
// string                <-- string
// time.Duration         <-- string
// time.Time             <-- string (only in RFC3339 format)
// IntOrStr              <-- string or int64 (only supports int32 values)
//
// (1) TODO: support other key types, such as numeric and attempt conversion from the string key
func Convert(value interface{}, target interface{}) error {
	if reflect.TypeOf(target).Kind() != reflect.Pointer {
		return fmt.Errorf("target must be a pointer")
	}

	targetValue := reflect.ValueOf(target).Elem()

	if !targetValue.CanSet() {
		return fmt.Errorf("target value cannot be set")
	}

	// check for special cases, handle default cases outside the switch
	//nolint:exhaustive
	switch targetValue.Kind() {
	case reflect.Map:
		return convertMap(value, target)
	case reflect.Slice:
		return convertSlice(value, target)
	case reflect.Struct:
		if targetValue.Type().String() == "time.Time" {
			return convertTime(value, target)
		}
		// default struct conversion
		return convertStruct(value, target)
	case reflect.Int64:
		if targetValue.Type().String() == "time.Duration" {
			return convertDuration(value, target)
		}

		if targetValue.Type().String() == "intstr.IntOrString" {
			return convertIntOrString(value, target)
		}
	case reflect.String:
		if targetValue.Type().String() == "intstr.IntOrString" {
			return convertIntOrString(value, target)
		}
	}

	// try default conversions
	// TODO: be more strict and disallow float to int conversions
	valueType := reflect.TypeOf(value)
	if !valueType.ConvertibleTo(targetValue.Type()) {
		return fmt.Errorf("expected %s got %s", targetValue.Type(), valueType)
	}
	targetValue.Set(reflect.ValueOf(value).Convert(targetValue.Type()))
	return nil
}

func convertMap(value interface{}, target interface{}) error {
	targetValue := reflect.ValueOf(target).Elem()

	// this can happen when converting a field of a struct (map fields are not initialized!)
	if targetValue.IsNil() {
		targetValue.Set(reflect.MakeMap(targetValue.Type()))
	}

	valueMap, ok := value.(map[string]interface{})
	if !ok {
		return fmt.Errorf("expected value map[string]interface{} got %s", reflect.TypeOf(value))
	}
	elemType := targetValue.Type().Elem()

	for mapKey, mapValue := range valueMap {
		targetItem := reflect.New(elemType)
		err := Convert(mapValue, targetItem.Interface())
		if err != nil {
			return err
		}
		targetValue.SetMapIndex(reflect.ValueOf(mapKey), targetItem.Elem())
	}

	return nil
}

func convertSlice(value interface{}, target interface{}) error {
	targetValue := reflect.ValueOf(target).Elem()

	valueSlice, ok := value.([]interface{})
	if !ok {
		return fmt.Errorf("expected value []interface{} got %s", reflect.TypeOf(value))
	}

	elemType := targetValue.Type().Elem()
	slice := targetValue
	for _, valueItem := range valueSlice {
		targetItem := reflect.New(elemType)
		err := Convert(valueItem, targetItem.Interface())
		if err != nil {
			return err
		}
		slice = reflect.Append(slice, targetItem.Elem())
	}

	targetValue.Set(slice)

	return nil
}

func convertStruct(value interface{}, target interface{}) error {
	targetValue := reflect.ValueOf(target).Elem()

	fieldMap, ok := value.(map[string]interface{})
	if !ok {
		return fmt.Errorf("expected value map[string]interface{} got %s", reflect.TypeOf(value))
	}

	for field, fieldValue := range fieldMap {
		sf := targetValue.FieldByName(toGoCase(field))
		if !sf.IsValid() {
			return fmt.Errorf("unknown field %s in struct %s", field, targetValue.Type().Name())
		}

		err := Convert(fieldValue, sf.Addr().Interface())
		if err != nil {
			return fmt.Errorf("error converting field %s of struct %s: %w", field, targetValue.Type().Name(), err)
		}
	}

	return nil
}

func convertDuration(value interface{}, target interface{}) error {
	targetValue := reflect.ValueOf(target).Elem()

	durationString := new(string)
	err := Convert(value, durationString)
	if err != nil {
		return err
	}

	duration, err := time.ParseDuration(*durationString)
	if err != nil {
		return err
	}

	targetValue.Set(reflect.ValueOf(duration))
	return nil
}

func convertTime(value interface{}, target interface{}) error {
	targetValue := reflect.ValueOf(target).Elem()

	timeString, ok := value.(string)
	if !ok {
		return fmt.Errorf("expected string value got %s", reflect.TypeOf(value))
	}

	timeValue, err := time.Parse(time.RFC3339, timeString)
	if err != nil {
		return err
	}

	targetValue.Set(reflect.ValueOf(timeValue))
	return nil
}

func convertIntOrString(value interface{}, target interface{}) error {
	targetValue := reflect.ValueOf(target).Elem()

	int64Value, ok := value.(int64)
	if ok {
		// check overflow here to avoid panic in the conversion
		if int64Value > math.MaxInt32 || int64Value < math.MinInt32 {
			return fmt.Errorf("value overflows int32 range: %d", int64Value)
		}
		intOrStrValue := intstr.FromInt32(int32(int64Value))
		targetValue.Set(reflect.ValueOf(intOrStrValue))
		return nil
	}

	stringValue, ok := value.(string)
	if ok {
		intOrStrValue := intstr.FromString(stringValue)
		targetValue.Set(reflect.ValueOf(intOrStrValue))
		return nil
	}

	return fmt.Errorf("expected int or string value got %s", reflect.TypeOf(value))
}
