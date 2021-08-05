// Copyright 2019 DeepMap, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package runtime

import (
	"fmt"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/deepmap/oapi-codegen/pkg/types"
)

// Parameter escaping works differently based on where a header is found
type ParamLocation int

const (
	ParamLocationUndefined ParamLocation = iota
	ParamLocationQuery
	ParamLocationPath
	ParamLocationHeader
	ParamLocationCookie
)

// This function is used by older generated code, and must remain compatible
// with that code. It is not to be used in new templates. Please see the
// function below, which can specialize its output based on the location of
// the parameter.
func StyleParam(style string, explode bool, paramName string, value interface{}) (string, error) {
	return StyleParamWithLocation(style, explode, paramName, ParamLocationUndefined, value)
}

// Given an input value, such as a primitive type, array or object, turn it
// into a parameter based on style/explode definition, performing whatever
// escaping is necessary based on parameter location
func StyleParamWithLocation(style string, explode bool, paramName string, paramLocation ParamLocation, value interface{}) (string, error) {
	t := reflect.TypeOf(value)
	v := reflect.ValueOf(value)

	// Things may be passed in by pointer, we need to dereference, so return
	// error on nil.
	if t.Kind() == reflect.Ptr {
		if v.IsNil() {
			return "", fmt.Errorf("value is a nil pointer")
		}
		v = reflect.Indirect(v)
		t = v.Type()
	}

	switch t.Kind() {
	case reflect.Slice:
		n := v.Len()
		sliceVal := make([]interface{}, n)
		for i := 0; i < n; i++ {
			sliceVal[i] = v.Index(i).Interface()
		}
		return styleSlice(style, explode, paramName, paramLocation, sliceVal)
	case reflect.Struct:
		return styleStruct(style, explode, paramName, paramLocation, value)
	case reflect.Map:
		return styleMap(style, explode, paramName, paramLocation, value)
	default:
		return stylePrimitive(style, explode, paramName, paramLocation, value)
	}
}

func styleSlice(style string, explode bool, paramName string, paramLocation ParamLocation, values []interface{}) (string, error) {
	if style == "deepObject" {
		if !explode {
			return "", errors.New("deepObjects must be exploded")
		}
		return MarshalDeepObject(values, paramName)
	}

	var prefix string
	var separator string

	switch style {
	case "simple":
		separator = ","
	case "label":
		prefix = "."
		if explode {
			separator = "."
		} else {
			separator = ","
		}
	case "matrix":
		prefix = fmt.Sprintf(";%s=", paramName)
		if explode {
			separator = prefix
		} else {
			separator = ","
		}
	case "form":
		prefix = fmt.Sprintf("%s=", paramName)
		if explode {
			separator = "&" + prefix
		} else {
			separator = ","
		}
	case "spaceDelimited":
		prefix = fmt.Sprintf("%s=", paramName)
		if explode {
			separator = "&" + prefix
		} else {
			separator = " "
		}
	case "pipeDelimited":
		prefix = fmt.Sprintf("%s=", paramName)
		if explode {
			separator = "&" + prefix
		} else {
			separator = "|"
		}
	default:
		return "", fmt.Errorf("unsupported style '%s'", style)
	}

	// We're going to assume here that the array is one of simple types.
	var err error
	var part string
	parts := make([]string, len(values))
	for i, v := range values {
		part, err = primitiveToString(v)
		part = escapeParameterString(part, paramLocation)
		parts[i] = part
		if err != nil {
			return "", fmt.Errorf("error formatting '%s': %s", paramName, err)
		}
	}
	return prefix + strings.Join(parts, separator), nil
}

func sortedKeys(strMap map[string]string) []string {
	keys := make([]string, len(strMap))
	i := 0
	for k := range strMap {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	return keys
}

// This is a special case. The struct may be a date or time, in
// which case, marshal it in correct format.
func marshalDateTimeValue(value interface{}) (string, bool) {
	v := reflect.Indirect(reflect.ValueOf(value))
	t := v.Type()

	if t.ConvertibleTo(reflect.TypeOf(time.Time{})) {
		tt := v.Convert(reflect.TypeOf(time.Time{}))
		timeVal := tt.Interface().(time.Time)
		return timeVal.Format(time.RFC3339Nano), true
	}

	if t.ConvertibleTo(reflect.TypeOf(types.Date{})) {
		d := v.Convert(reflect.TypeOf(types.Date{}))
		dateVal := d.Interface().(types.Date)
		return dateVal.Format(types.DateFormat), true
	}

	return "", false
}

func styleStruct(style string, explode bool, paramName string, paramLocation ParamLocation, value interface{}) (string, error) {

	if timeVal, ok := marshalDateTimeValue(value); ok {
		styledVal, err := stylePrimitive(style, explode, paramName, paramLocation, timeVal)
		if err != nil {
			return "", errors.Wrap(err, "failed to style time")
		}
		return styledVal, nil
	}

	if style == "deepObject" {
		if !explode {
			return "", errors.New("deepObjects must be exploded")
		}
		return MarshalDeepObject(value, paramName)
	}

	// Otherwise, we need to build a dictionary of the struct's fields. Each
	// field may only be a primitive value.
	v := reflect.ValueOf(value)
	t := reflect.TypeOf(value)
	fieldDict := make(map[string]string)

	for i := 0; i < t.NumField(); i++ {
		fieldT := t.Field(i)
		// Find the json annotation on the field, and use the json specified
		// name if available, otherwise, just the field name.
		tag := fieldT.Tag.Get("json")
		fieldName := fieldT.Name
		if tag != "" {
			tagParts := strings.Split(tag, ",")
			name := tagParts[0]
			if name != "" {
				fieldName = name
			}
		}
		f := v.Field(i)

		// Unset optional fields will be nil pointers, skip over those.
		if f.Type().Kind() == reflect.Ptr && f.IsNil() {
			continue
		}
		str, err := primitiveToString(f.Interface())
		if err != nil {
			return "", fmt.Errorf("error formatting '%s': %s", paramName, err)
		}
		fieldDict[fieldName] = str
	}

	return processFieldDict(style, explode, paramName, paramLocation, fieldDict)
}

func styleMap(style string, explode bool, paramName string, paramLocation ParamLocation, value interface{}) (string, error) {
	if style == "deepObject" {
		if !explode {
			return "", errors.New("deepObjects must be exploded")
		}
		return MarshalDeepObject(value, paramName)
	}

	dict, ok := value.(map[string]interface{})
	if !ok {
		return "", errors.New("map not of type map[string]interface{}")
	}

	fieldDict := make(map[string]string)
	for fieldName, value := range dict {
		str, err := primitiveToString(value)
		if err != nil {
			return "", fmt.Errorf("error formatting '%s': %s", paramName, err)
		}
		fieldDict[fieldName] = str
	}
	return processFieldDict(style, explode, paramName, paramLocation, fieldDict)
}

func processFieldDict(style string, explode bool, paramName string, paramLocation ParamLocation, fieldDict map[string]string) (string, error) {
	var parts []string

	// This works for everything except deepObject. We'll handle that one
	// separately.
	if style != "deepObject" {
		if explode {
			for _, k := range sortedKeys(fieldDict) {
				v := escapeParameterString(fieldDict[k], paramLocation)
				parts = append(parts, k+"="+v)
			}
		} else {
			for _, k := range sortedKeys(fieldDict) {
				v := escapeParameterString(fieldDict[k], paramLocation)
				parts = append(parts, k)
				parts = append(parts, v)
			}
		}
	}

	var prefix string
	var separator string

	switch style {
	case "simple":
		separator = ","
	case "label":
		prefix = "."
		if explode {
			separator = prefix
		} else {
			separator = ","
		}
	case "matrix":
		if explode {
			separator = ";"
			prefix = ";"
		} else {
			separator = ","
			prefix = fmt.Sprintf(";%s=", paramName)
		}
	case "form":
		if explode {
			separator = "&"
		} else {
			prefix = fmt.Sprintf("%s=", paramName)
			separator = ","
		}
	case "deepObject":
		{
			if !explode {
				return "", fmt.Errorf("deepObject parameters must be exploded")
			}
			for _, k := range sortedKeys(fieldDict) {
				v := fieldDict[k]
				part := fmt.Sprintf("%s[%s]=%s", paramName, k, v)
				parts = append(parts, part)
			}
			separator = "&"
		}
	default:
		return "", fmt.Errorf("unsupported style '%s'", style)
	}

	return prefix + strings.Join(parts, separator), nil
}

func stylePrimitive(style string, explode bool, paramName string, paramLocation ParamLocation, value interface{}) (string, error) {
	strVal, err := primitiveToString(value)
	if err != nil {
		return "", err
	}

	var prefix string
	switch style {
	case "simple":
	case "label":
		prefix = "."
	case "matrix":
		prefix = fmt.Sprintf(";%s=", paramName)
	case "form":
		prefix = fmt.Sprintf("%s=", paramName)
	default:
		return "", fmt.Errorf("unsupported style '%s'", style)
	}
	return prefix + escapeParameterString(strVal, paramLocation), nil
}

// Converts a primitive value to a string. We need to do this based on the
// Kind of an interface, not the Type to work with aliased types.
func primitiveToString(value interface{}) (string, error) {
	var output string

	// Values may come in by pointer for optionals, so make sure to dereferene.
	v := reflect.Indirect(reflect.ValueOf(value))
	t := v.Type()
	kind := t.Kind()

	switch kind {
	case reflect.Int8, reflect.Int32, reflect.Int64, reflect.Int:
		output = strconv.FormatInt(v.Int(), 10)
	case reflect.Float64:
		output = strconv.FormatFloat(v.Float(), 'f', -1, 64)
	case reflect.Float32:
		output = strconv.FormatFloat(v.Float(), 'f', -1, 32)
	case reflect.Bool:
		if v.Bool() {
			output = "true"
		} else {
			output = "false"
		}
	case reflect.String:
		output = v.String()
	default:
		return "", fmt.Errorf("unsupported type %s", reflect.TypeOf(value).String())
	}
	return output, nil
}

// This function escapes a parameter value bas on the location of that parameter.
// Query params and path params need different kinds of escaping, while header
// and cookie params seem not to need escaping.
func escapeParameterString(value string, paramLocation ParamLocation) string {
	switch paramLocation {
	case ParamLocationQuery:
		return url.QueryEscape(value)
	case ParamLocationPath:
		return url.PathEscape(value)
	default:
		return value
	}
}
