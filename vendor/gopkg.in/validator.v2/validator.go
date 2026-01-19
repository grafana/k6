// Package validator implements value validations
//
// Copyright 2014 Roberto Teixeira <robteix@robteix.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validator

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// TextErr is an error that also implements the TextMarshaller interface for
// serializing out to various plain text encodings. Packages creating their
// own custom errors should use TextErr if they're intending to use serializing
// formats like json, msgpack etc.
type TextErr struct {
	Err error
}

// Error implements the error interface.
func (t TextErr) Error() string {
	return t.Err.Error()
}

// MarshalText implements the TextMarshaller
func (t TextErr) MarshalText() ([]byte, error) {
	return []byte(t.Err.Error()), nil
}

var (
	// ErrZeroValue is the error returned when variable has zero value
	// and nonzero or nonnil was specified
	ErrZeroValue = TextErr{errors.New("zero value")}
	// ErrMin is the error returned when variable is less than mininum
	// value specified
	ErrMin = TextErr{errors.New("less than min")}
	// ErrMax is the error returned when variable is more than
	// maximum specified
	ErrMax = TextErr{errors.New("greater than max")}
	// ErrLen is the error returned when length is not equal to
	// param specified
	ErrLen = TextErr{errors.New("invalid length")}
	// ErrRegexp is the error returned when the value does not
	// match the provided regular expression parameter
	ErrRegexp = TextErr{errors.New("regular expression mismatch")}
	// ErrUnsupported is the error error returned when a validation rule
	// is used with an unsupported variable type
	ErrUnsupported = TextErr{errors.New("unsupported type")}
	// ErrBadParameter is the error returned when an invalid parameter
	// is provided to a validation rule (e.g. a string where an int was
	// expected (max=foo,len=bar) or missing a parameter when one is required (len=))
	ErrBadParameter = TextErr{errors.New("bad parameter")}
	// ErrUnknownTag is the error returned when an unknown tag is found
	ErrUnknownTag = TextErr{errors.New("unknown tag")}
	// ErrInvalid is the error returned when variable is invalid
	// (normally a nil pointer)
	ErrInvalid = TextErr{errors.New("invalid value")}
	// ErrCannotValidate is the error returned when a struct is unexported
	ErrCannotValidate = TextErr{errors.New("cannot validate unexported struct")}
)

// ErrorMap is a map which contains all errors from validating a struct.
type ErrorMap map[string]ErrorArray

// ErrorMap implements the Error interface so we can check error against nil.
// The returned error is all existing errors with the map.
func (err ErrorMap) Error() string {
	var b bytes.Buffer

	for k, errs := range err {
		if len(errs) > 0 {
			b.WriteString(fmt.Sprintf("%s: %s, ", k, errs.Error()))
		}
	}

	return strings.TrimSuffix(b.String(), ", ")
}

// ErrorArray is a slice of errors returned by the Validate function.
type ErrorArray []error

// ErrorArray implements the Error interface and returns all the errors comma seprated
// if errors exist.
func (err ErrorArray) Error() string {
	var b bytes.Buffer

	for _, errs := range err {
		b.WriteString(fmt.Sprintf("%s, ", errs.Error()))
	}

	errs := b.String()
	return strings.TrimSuffix(errs, ", ")
}

// ValidationFunc is a function that receives the value of a
// field and a parameter used for the respective validation tag.
type ValidationFunc func(v interface{}, param string) error

// Validator implements a validator
type Validator struct {
	// validationFuncs is a map of ValidationFuncs indexed
	// by their name.
	validationFuncs map[string]ValidationFunc
	// Tag name being used.
	tagName string
	// printJSON set to true will make errors print with the
	// name of their json field instead of their struct tag.
	// If no json tag is present the name of the struct field is used.
	printJSON bool
}

// Helper validator so users can use the
// functions directly from the package
var defaultValidator = NewValidator()

// NewValidator creates a new Validator
func NewValidator() *Validator {
	return &Validator{
		tagName: "validate",
		validationFuncs: map[string]ValidationFunc{
			"nonzero": nonzero,
			"len":     length,
			"min":     min,
			"max":     max,
			"regexp":  regex,
			"nonnil":  nonnil,
		},
		printJSON: false,
	}
}

// SetTag allows you to change the tag name used in structs
func SetTag(tag string) {
	defaultValidator.SetTag(tag)
}

// SetTag allows you to change the tag name used in structs
func (mv *Validator) SetTag(tag string) {
	mv.tagName = tag
}

// WithTag creates a new Validator with the new tag name. It is
// useful to chain-call with Validate so we don't change the tag
// name permanently: validator.WithTag("foo").Validate(t)
func WithTag(tag string) *Validator {
	return defaultValidator.WithTag(tag)
}

// WithTag creates a new Validator with the new tag name. It is
// useful to chain-call with Validate so we don't change the tag
// name permanently: validator.WithTag("foo").Validate(t)
func (mv *Validator) WithTag(tag string) *Validator {
	v := mv.copy()
	v.SetTag(tag)
	return v
}

// SetPrintJSON allows you to print errors with json tag names present in struct tags
func SetPrintJSON(printJSON bool) {
	defaultValidator.SetPrintJSON(printJSON)
}

// SetPrintJSON allows you to print errors with json tag names present in struct tags
func (mv *Validator) SetPrintJSON(printJSON bool) {
	mv.printJSON = printJSON
}

// WithPrintJSON creates a new Validator with printJSON set to new value. It is
// useful to chain-call with Validate so we don't change the print option
// permanently: validator.WithPrintJSON(true).Validate(t)
func WithPrintJSON(printJSON bool) *Validator {
	return defaultValidator.WithPrintJSON(printJSON)
}

// WithPrintJSON creates a new Validator with printJSON set to new value. It is
// useful to chain-call with Validate so we don't change the print option
// permanently: validator.WithTag("foo").WithPrintJSON(true).Validate(t)
func (mv *Validator) WithPrintJSON(printJSON bool) *Validator {
	v := mv.copy()
	v.SetPrintJSON(printJSON)
	return v
}

// Copy a validator
func (mv *Validator) copy() *Validator {
	newFuncs := map[string]ValidationFunc{}
	for k, f := range mv.validationFuncs {
		newFuncs[k] = f
	}
	return &Validator{
		tagName:         mv.tagName,
		validationFuncs: newFuncs,
		printJSON:       mv.printJSON,
	}
}

// SetValidationFunc sets the function to be used for a given
// validation constraint. Calling this function with nil vf
// is the same as removing the constraint function from the list.
func SetValidationFunc(name string, vf ValidationFunc) error {
	return defaultValidator.SetValidationFunc(name, vf)
}

// SetValidationFunc sets the function to be used for a given
// validation constraint. Calling this function with nil vf
// is the same as removing the constraint function from the list.
func (mv *Validator) SetValidationFunc(name string, vf ValidationFunc) error {
	if name == "" {
		return errors.New("name cannot be empty")
	}
	if vf == nil {
		delete(mv.validationFuncs, name)
		return nil
	}
	mv.validationFuncs[name] = vf
	return nil
}

// Validate calls the Validate method on the default validator.
func Validate(v interface{}) error {
	return defaultValidator.Validate(v)
}

// Validate validates the fields of structs (included embedded structs) based on
// 'validator' tags and returns errors found indexed by the field name.
func (mv *Validator) Validate(v interface{}) error {
	m := make(ErrorMap)
	mv.deepValidateCollection(reflect.ValueOf(v), m, func() string {
		return ""
	})
	if len(m) > 0 {
		return m
	}
	return nil
}

func (mv *Validator) validateStruct(sv reflect.Value, m ErrorMap) error {
	kind := sv.Kind()
	if (kind == reflect.Ptr || kind == reflect.Interface) && !sv.IsNil() {
		return mv.validateStruct(sv.Elem(), m)
	}
	if kind != reflect.Struct && kind != reflect.Interface {
		return ErrUnsupported
	}

	st := sv.Type()
	nfields := st.NumField()
	for i := 0; i < nfields; i++ {
		if err := mv.validateField(st.Field(i), sv.Field(i), m); err != nil {
			return err
		}
	}

	return nil
}

// validateField validates the field of fieldVal referred to by fieldDef.
// If fieldDef refers to an anonymous/embedded field,
// validateField will walk all of the embedded type's fields and validate them on sv.
func (mv *Validator) validateField(fieldDef reflect.StructField, fieldVal reflect.Value, m ErrorMap) error {
	tag := fieldDef.Tag.Get(mv.tagName)
	if tag == "-" {
		return nil
	}
	// deal with pointers
	for (fieldVal.Kind() == reflect.Ptr || fieldVal.Kind() == reflect.Interface) && !fieldVal.IsNil() {
		fieldVal = fieldVal.Elem()
	}

	// ignore private structs unless Anonymous
	if !fieldDef.Anonymous && fieldDef.PkgPath != "" {
		return nil
	}

	var errs ErrorArray
	if tag != "" {
		var err error
		if fieldDef.PkgPath != "" {
			err = ErrCannotValidate
		} else {
			err = mv.validValue(fieldVal, tag)
		}
		if errarr, ok := err.(ErrorArray); ok {
			errs = errarr
		} else if err != nil {
			errs = ErrorArray{err}
		}
	}

	// no-op if field is not a struct, interface, array, slice or map
	fn := mv.fieldName(fieldDef)
	mv.deepValidateCollection(fieldVal, m, func() string {
		return fn
	})

	if len(errs) > 0 {
		m[fn] = errs
	}
	return nil
}

func (mv *Validator) fieldName(fieldDef reflect.StructField) string {
	if mv.printJSON {
		if jsonTagValue, ok := fieldDef.Tag.Lookup("json"); ok {
			return parseName(jsonTagValue)
		}
	}
	return fieldDef.Name
}

func (mv *Validator) deepValidateCollection(f reflect.Value, m ErrorMap, fnameFn func() string) {
	switch f.Kind() {
	case reflect.Interface, reflect.Ptr:
		if f.IsNil() {
			return
		}
		mv.deepValidateCollection(f.Elem(), m, fnameFn)
	case reflect.Struct:
		subm := make(ErrorMap)
		err := mv.validateStruct(f, subm)
		parentName := fnameFn()
		if err != nil {
			m[parentName] = ErrorArray{err}
		}
		for j, k := range subm {
			keyName := j
			if parentName != "" {
				keyName = parentName + "." + keyName
			}
			m[keyName] = k
		}
	case reflect.Array, reflect.Slice:
		// we don't need to loop over every byte in a byte slice so we only end up
		// looping when the kind is something we care about
		switch f.Type().Elem().Kind() {
		case reflect.Struct, reflect.Interface, reflect.Ptr, reflect.Map, reflect.Array, reflect.Slice:
			for i := 0; i < f.Len(); i++ {
				mv.deepValidateCollection(f.Index(i), m, func() string {
					return fmt.Sprintf("%s[%d]", fnameFn(), i)
				})
			}
		}
	case reflect.Map:
		for _, key := range f.MapKeys() {
			mv.deepValidateCollection(key, m, func() string {
				return fmt.Sprintf("%s[%+v](key)", fnameFn(), key.Interface())
			}) // validate the map key
			value := f.MapIndex(key)
			mv.deepValidateCollection(value, m, func() string {
				return fmt.Sprintf("%s[%+v](value)", fnameFn(), key.Interface())
			})
		}
	}
}

// Valid validates a value based on the provided
// tags and returns errors found or nil.
func Valid(val interface{}, tags string) error {
	return defaultValidator.Valid(val, tags)
}

// Valid validates a value based on the provided
// tags and returns errors found or nil.
func (mv *Validator) Valid(val interface{}, tags string) error {
	if tags == "-" {
		return nil
	}
	v := reflect.ValueOf(val)
	if (v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface) && !v.IsNil() {
		return mv.validValue(v.Elem(), tags)
	}
	if v.Kind() == reflect.Invalid {
		return mv.validateVar(nil, tags)
	}
	return mv.validateVar(val, tags)
}

// validValue is like Valid but takes a Value instead of an interface
func (mv *Validator) validValue(v reflect.Value, tags string) error {
	if v.Kind() == reflect.Invalid {
		return mv.validateVar(nil, tags)
	}
	return mv.validateVar(v.Interface(), tags)
}

// validateVar validates one single variable
func (mv *Validator) validateVar(v interface{}, tag string) error {
	tags, err := mv.parseTags(tag)
	if err != nil {
		// unknown tag found, give up.
		return err
	}
	errs := make(ErrorArray, 0, len(tags))
	for _, t := range tags {
		if err := t.Fn(v, t.Param); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// tag represents one of the tag items
type tag struct {
	Name  string         // name of the tag
	Fn    ValidationFunc // validation function to call
	Param string         // parameter to send to the validation function
}

// separate by no escaped commas
var sepPattern *regexp.Regexp = regexp.MustCompile(`((?:^|[^\\])(?:\\\\)*),`)

func splitUnescapedComma(str string) []string {
	ret := []string{}
	indexes := sepPattern.FindAllStringIndex(str, -1)
	last := 0
	for _, is := range indexes {
		ret = append(ret, str[last:is[1]-1])
		last = is[1]
	}
	ret = append(ret, str[last:])
	return ret
}

// parseTags parses all individual tags found within a struct tag.
func (mv *Validator) parseTags(t string) ([]tag, error) {
	tl := splitUnescapedComma(t)
	tags := make([]tag, 0, len(tl))
	for _, i := range tl {
		i = strings.Replace(i, `\,`, ",", -1)
		tg := tag{}
		v := strings.SplitN(i, "=", 2)
		tg.Name = strings.Trim(v[0], " ")
		if tg.Name == "" {
			return []tag{}, ErrUnknownTag
		}
		if len(v) > 1 {
			tg.Param = strings.Trim(v[1], " ")
		}
		var found bool
		if tg.Fn, found = mv.validationFuncs[tg.Name]; !found {
			return []tag{}, ErrUnknownTag
		}
		tags = append(tags, tg)

	}
	return tags, nil
}

func parseName(tag string) string {
	if tag == "" {
		return ""
	}

	name := strings.SplitN(tag, ",", 2)[0]

	// if the field as be skipped in json, just return an empty string
	if name == "-" {
		return ""
	}
	return name
}
