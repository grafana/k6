package blackmagic

import (
	"fmt"
	"reflect"
)

// AssignField is a convenience function to assign a value to
// an optional struct field. In Go, an optional struct field is
// usually denoted by a pointer to T instead of T:
//
//	type Object struct {
//	  Optional *T
//	}
//
// This gets a bit cumbersome when you want to assign literals
// or you do not want to worry about taking the address of a
// variable.
//
//	Object.Optional = &"foo" // doesn't compile!
//
// Instead you can use this function to do it in one line:
//
//	blackmagic.AssignOptionalField(&Object.Optionl, "foo")
func AssignOptionalField(dst, src interface{}) error {
	dstRV := reflect.ValueOf(dst)
	srcRV := reflect.ValueOf(src)
	if dstRV.Kind() != reflect.Pointer || dstRV.Elem().Kind() != reflect.Pointer {
		return fmt.Errorf(`dst must be a pointer to a field that is turn a pointer of src (%T)`, src)
	}

	if !dstRV.Elem().CanSet() {
		return fmt.Errorf(`dst (%T) is not assignable`, dstRV.Elem().Interface())
	}
	if !reflect.PtrTo(srcRV.Type()).AssignableTo(dstRV.Elem().Type()) {
		return fmt.Errorf(`cannot assign src (%T) to dst (%T)`, src, dst)
	}

	ptr := reflect.New(srcRV.Type())
	ptr.Elem().Set(srcRV)
	dstRV.Elem().Set(ptr)
	return nil
}

// AssignIfCompatible is a convenience function to safely
// assign arbitrary values. dst must be a pointer to an
// empty interface, or it must be a pointer to a compatible
// variable type that can hold src.
func AssignIfCompatible(dst, src interface{}) error {
	orv := reflect.ValueOf(src) // save this value for error reporting
	result := orv

	// t can be a pointer or a slice, and the code will slightly change
	// depending on this
	var isPtr bool
	var isSlice bool
	switch result.Kind() {
	case reflect.Ptr:
		isPtr = true
	case reflect.Slice:
		isSlice = true
	}

	rv := reflect.ValueOf(dst)
	if rv.Kind() != reflect.Ptr {
		return fmt.Errorf(`destination argument to AssignIfCompatible() must be a pointer: %T`, dst)
	}

	actualDst := rv.Elem()
	switch actualDst.Kind() {
	case reflect.Interface:
		// If it's an interface, we can just assign the pointer to the interface{}
	default:
		// If it's a pointer to the struct we're looking for, we need to set
		// the de-referenced struct
		if !isSlice && isPtr {
			result = result.Elem()
		}
	}
	if !result.Type().AssignableTo(actualDst.Type()) {
		return fmt.Errorf(`argument to AssignIfCompatible() must be compatible with %T (was %T)`, orv.Interface(), dst)
	}

	if !actualDst.CanSet() {
		return fmt.Errorf(`argument to AssignIfCompatible() must be settable`)
	}
	actualDst.Set(result)

	return nil
}
