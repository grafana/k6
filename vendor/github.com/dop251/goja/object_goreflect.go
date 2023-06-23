package goja

import (
	"fmt"
	"go/ast"
	"reflect"
	"strings"

	"github.com/dop251/goja/parser"
	"github.com/dop251/goja/unistring"
)

// JsonEncodable allows custom JSON encoding by JSON.stringify()
// Note that if the returned value itself also implements JsonEncodable, it won't have any effect.
type JsonEncodable interface {
	JsonEncodable() interface{}
}

// FieldNameMapper provides custom mapping between Go and JavaScript property names.
type FieldNameMapper interface {
	// FieldName returns a JavaScript name for the given struct field in the given type.
	// If this method returns "" the field becomes hidden.
	FieldName(t reflect.Type, f reflect.StructField) string

	// MethodName returns a JavaScript name for the given method in the given type.
	// If this method returns "" the method becomes hidden.
	MethodName(t reflect.Type, m reflect.Method) string
}

type tagFieldNameMapper struct {
	tagName      string
	uncapMethods bool
}

func (tfm tagFieldNameMapper) FieldName(_ reflect.Type, f reflect.StructField) string {
	tag := f.Tag.Get(tfm.tagName)
	if idx := strings.IndexByte(tag, ','); idx != -1 {
		tag = tag[:idx]
	}
	if parser.IsIdentifier(tag) {
		return tag
	}
	return ""
}

func uncapitalize(s string) string {
	return strings.ToLower(s[0:1]) + s[1:]
}

func (tfm tagFieldNameMapper) MethodName(_ reflect.Type, m reflect.Method) string {
	if tfm.uncapMethods {
		return uncapitalize(m.Name)
	}
	return m.Name
}

type uncapFieldNameMapper struct {
}

func (u uncapFieldNameMapper) FieldName(_ reflect.Type, f reflect.StructField) string {
	return uncapitalize(f.Name)
}

func (u uncapFieldNameMapper) MethodName(_ reflect.Type, m reflect.Method) string {
	return uncapitalize(m.Name)
}

type reflectFieldInfo struct {
	Index     []int
	Anonymous bool
}

type reflectFieldsInfo struct {
	Fields map[string]reflectFieldInfo
	Names  []string
}

type reflectMethodsInfo struct {
	Methods map[string]int
	Names   []string
}

type reflectValueWrapper interface {
	esValue() Value
	reflectValue() reflect.Value
	setReflectValue(reflect.Value)
}

func isContainer(k reflect.Kind) bool {
	switch k {
	case reflect.Struct, reflect.Slice, reflect.Array:
		return true
	}
	return false
}

func copyReflectValueWrapper(w reflectValueWrapper) {
	v := w.reflectValue()
	c := reflect.New(v.Type()).Elem()
	c.Set(v)
	w.setReflectValue(c)
}

type objectGoReflect struct {
	baseObject
	origValue, fieldsValue reflect.Value

	fieldsInfo  *reflectFieldsInfo
	methodsInfo *reflectMethodsInfo

	methodsValue reflect.Value

	valueCache map[string]reflectValueWrapper

	toString, valueOf func() Value

	toJson func() interface{}
}

func (o *objectGoReflect) init() {
	o.baseObject.init()
	switch o.fieldsValue.Kind() {
	case reflect.Bool:
		o.class = classBoolean
		o.prototype = o.val.runtime.global.BooleanPrototype
		o.toString = o._toStringBool
		o.valueOf = o._valueOfBool
	case reflect.String:
		o.class = classString
		o.prototype = o.val.runtime.global.StringPrototype
		o.toString = o._toStringString
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		o.class = classNumber
		o.prototype = o.val.runtime.global.NumberPrototype
		o.valueOf = o._valueOfInt
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		o.class = classNumber
		o.prototype = o.val.runtime.global.NumberPrototype
		o.valueOf = o._valueOfUint
	case reflect.Float32, reflect.Float64:
		o.class = classNumber
		o.prototype = o.val.runtime.global.NumberPrototype
		o.valueOf = o._valueOfFloat
	default:
		o.class = classObject
		o.prototype = o.val.runtime.global.ObjectPrototype
	}

	if o.fieldsValue.Kind() == reflect.Struct {
		o.fieldsInfo = o.val.runtime.fieldsInfo(o.fieldsValue.Type())
	}

	var methodsType reflect.Type
	// Always use pointer type for non-interface values to be able to access both methods defined on
	// the literal type and on the pointer.
	if o.fieldsValue.Kind() != reflect.Interface {
		methodsType = reflect.PtrTo(o.fieldsValue.Type())
	} else {
		methodsType = o.fieldsValue.Type()
	}

	o.methodsInfo = o.val.runtime.methodsInfo(methodsType)

	// Container values and values that have at least one method defined on the pointer type
	// need to be addressable.
	if !o.origValue.CanAddr() && (isContainer(o.origValue.Kind()) || len(o.methodsInfo.Names) > 0) {
		value := reflect.New(o.origValue.Type()).Elem()
		value.Set(o.origValue)
		o.origValue = value
		if value.Kind() != reflect.Ptr {
			o.fieldsValue = value
		}
	}

	o.extensible = true

	switch o.origValue.Interface().(type) {
	case fmt.Stringer:
		o.toString = o._toStringStringer
	case error:
		o.toString = o._toStringError
	}

	if len(o.methodsInfo.Names) > 0 && o.fieldsValue.Kind() != reflect.Interface {
		o.methodsValue = o.fieldsValue.Addr()
	} else {
		o.methodsValue = o.fieldsValue
	}

	if j, ok := o.origValue.Interface().(JsonEncodable); ok {
		o.toJson = j.JsonEncodable
	}
}

func (o *objectGoReflect) getStr(name unistring.String, receiver Value) Value {
	if v := o._get(name.String()); v != nil {
		return v
	}
	return o.baseObject.getStr(name, receiver)
}

func (o *objectGoReflect) _getField(jsName string) reflect.Value {
	if o.fieldsInfo != nil {
		if info, exists := o.fieldsInfo.Fields[jsName]; exists {
			return o.fieldsValue.FieldByIndex(info.Index)
		}
	}

	return reflect.Value{}
}

func (o *objectGoReflect) _getMethod(jsName string) reflect.Value {
	if o.methodsInfo != nil {
		if idx, exists := o.methodsInfo.Methods[jsName]; exists {
			return o.methodsValue.Method(idx)
		}
	}

	return reflect.Value{}
}

func (o *objectGoReflect) elemToValue(ev reflect.Value) (Value, reflectValueWrapper) {
	if isContainer(ev.Kind()) {
		ret := o.val.runtime.toValue(ev.Interface(), ev)
		if obj, ok := ret.(*Object); ok {
			if w, ok := obj.self.(reflectValueWrapper); ok {
				return ret, w
			}
		}
		return ret, nil
	}

	if ev.Kind() == reflect.Interface {
		ev = ev.Elem()
	}

	if ev.Kind() == reflect.Invalid {
		return _null, nil
	}

	return o.val.runtime.toValue(ev.Interface(), ev), nil
}

func (o *objectGoReflect) _getFieldValue(name string) Value {
	if v := o.valueCache[name]; v != nil {
		return v.esValue()
	}
	if v := o._getField(name); v.IsValid() {
		res, w := o.elemToValue(v)
		if w != nil {
			if o.valueCache == nil {
				o.valueCache = make(map[string]reflectValueWrapper)
			}
			o.valueCache[name] = w
		}
		return res
	}
	return nil
}

func (o *objectGoReflect) _get(name string) Value {
	if o.fieldsValue.Kind() == reflect.Struct {
		if ret := o._getFieldValue(name); ret != nil {
			return ret
		}
	}

	if v := o._getMethod(name); v.IsValid() {
		return o.val.runtime.toValue(v.Interface(), v)
	}

	return nil
}

func (o *objectGoReflect) getOwnPropStr(name unistring.String) Value {
	n := name.String()
	if o.fieldsValue.Kind() == reflect.Struct {
		if v := o._getFieldValue(n); v != nil {
			return &valueProperty{
				value:      v,
				writable:   true,
				enumerable: true,
			}
		}
	}

	if v := o._getMethod(n); v.IsValid() {
		return &valueProperty{
			value:      o.val.runtime.toValue(v.Interface(), v),
			enumerable: true,
		}
	}

	return o.baseObject.getOwnPropStr(name)
}

func (o *objectGoReflect) setOwnStr(name unistring.String, val Value, throw bool) bool {
	has, ok := o._put(name.String(), val, throw)
	if !has {
		if res, ok := o._setForeignStr(name, nil, val, o.val, throw); !ok {
			o.val.runtime.typeErrorResult(throw, "Cannot assign to property %s of a host object", name)
			return false
		} else {
			return res
		}
	}
	return ok
}

func (o *objectGoReflect) setForeignStr(name unistring.String, val, receiver Value, throw bool) (bool, bool) {
	return o._setForeignStr(name, trueValIfPresent(o._has(name.String())), val, receiver, throw)
}

func (o *objectGoReflect) setForeignIdx(idx valueInt, val, receiver Value, throw bool) (bool, bool) {
	return o._setForeignIdx(idx, nil, val, receiver, throw)
}

func (o *objectGoReflect) _put(name string, val Value, throw bool) (has, ok bool) {
	if o.fieldsValue.Kind() == reflect.Struct {
		if v := o._getField(name); v.IsValid() {
			cached := o.valueCache[name]
			if cached != nil {
				copyReflectValueWrapper(cached)
			}

			err := o.val.runtime.toReflectValue(val, v, &objectExportCtx{})
			if err != nil {
				if cached != nil {
					cached.setReflectValue(v)
				}
				o.val.runtime.typeErrorResult(throw, "Go struct conversion error: %v", err)
				return true, false
			}
			if cached != nil {
				delete(o.valueCache, name)
			}
			return true, true
		}
	}
	return false, false
}

func (o *objectGoReflect) _putProp(name unistring.String, value Value, writable, enumerable, configurable bool) Value {
	if _, ok := o._put(name.String(), value, false); ok {
		return value
	}
	return o.baseObject._putProp(name, value, writable, enumerable, configurable)
}

func (r *Runtime) checkHostObjectPropertyDescr(name unistring.String, descr PropertyDescriptor, throw bool) bool {
	if descr.Getter != nil || descr.Setter != nil {
		r.typeErrorResult(throw, "Host objects do not support accessor properties")
		return false
	}
	if descr.Writable == FLAG_FALSE {
		r.typeErrorResult(throw, "Host object field %s cannot be made read-only", name)
		return false
	}
	if descr.Configurable == FLAG_TRUE {
		r.typeErrorResult(throw, "Host object field %s cannot be made configurable", name)
		return false
	}
	return true
}

func (o *objectGoReflect) defineOwnPropertyStr(name unistring.String, descr PropertyDescriptor, throw bool) bool {
	if o.val.runtime.checkHostObjectPropertyDescr(name, descr, throw) {
		n := name.String()
		if has, ok := o._put(n, descr.Value, throw); !has {
			o.val.runtime.typeErrorResult(throw, "Cannot define property '%s' on a host object", n)
			return false
		} else {
			return ok
		}
	}
	return false
}

func (o *objectGoReflect) _has(name string) bool {
	if o.fieldsValue.Kind() == reflect.Struct {
		if v := o._getField(name); v.IsValid() {
			return true
		}
	}
	if v := o._getMethod(name); v.IsValid() {
		return true
	}
	return false
}

func (o *objectGoReflect) hasOwnPropertyStr(name unistring.String) bool {
	return o._has(name.String()) || o.baseObject.hasOwnPropertyStr(name)
}

func (o *objectGoReflect) _valueOfInt() Value {
	return intToValue(o.fieldsValue.Int())
}

func (o *objectGoReflect) _valueOfUint() Value {
	return intToValue(int64(o.fieldsValue.Uint()))
}

func (o *objectGoReflect) _valueOfBool() Value {
	if o.fieldsValue.Bool() {
		return valueTrue
	} else {
		return valueFalse
	}
}

func (o *objectGoReflect) _valueOfFloat() Value {
	return floatToValue(o.fieldsValue.Float())
}

func (o *objectGoReflect) _toStringStringer() Value {
	return newStringValue(o.origValue.Interface().(fmt.Stringer).String())
}

func (o *objectGoReflect) _toStringString() Value {
	return newStringValue(o.fieldsValue.String())
}

func (o *objectGoReflect) _toStringBool() Value {
	if o.fieldsValue.Bool() {
		return stringTrue
	} else {
		return stringFalse
	}
}

func (o *objectGoReflect) _toStringError() Value {
	return newStringValue(o.origValue.Interface().(error).Error())
}

func (o *objectGoReflect) deleteStr(name unistring.String, throw bool) bool {
	n := name.String()
	if o._has(n) {
		o.val.runtime.typeErrorResult(throw, "Cannot delete property %s from a Go type", n)
		return false
	}
	return o.baseObject.deleteStr(name, throw)
}

type goreflectPropIter struct {
	o   *objectGoReflect
	idx int
}

func (i *goreflectPropIter) nextField() (propIterItem, iterNextFunc) {
	names := i.o.fieldsInfo.Names
	if i.idx < len(names) {
		name := names[i.idx]
		i.idx++
		return propIterItem{name: newStringValue(name), enumerable: _ENUM_TRUE}, i.nextField
	}

	i.idx = 0
	return i.nextMethod()
}

func (i *goreflectPropIter) nextMethod() (propIterItem, iterNextFunc) {
	names := i.o.methodsInfo.Names
	if i.idx < len(names) {
		name := names[i.idx]
		i.idx++
		return propIterItem{name: newStringValue(name), enumerable: _ENUM_TRUE}, i.nextMethod
	}

	return propIterItem{}, nil
}

func (o *objectGoReflect) iterateStringKeys() iterNextFunc {
	r := &goreflectPropIter{
		o: o,
	}
	if o.fieldsInfo != nil {
		return r.nextField
	}

	return r.nextMethod
}

func (o *objectGoReflect) stringKeys(_ bool, accum []Value) []Value {
	// all own keys are enumerable
	if o.fieldsInfo != nil {
		for _, name := range o.fieldsInfo.Names {
			accum = append(accum, newStringValue(name))
		}
	}

	for _, name := range o.methodsInfo.Names {
		accum = append(accum, newStringValue(name))
	}

	return accum
}

func (o *objectGoReflect) export(*objectExportCtx) interface{} {
	return o.origValue.Interface()
}

func (o *objectGoReflect) exportType() reflect.Type {
	return o.origValue.Type()
}

func (o *objectGoReflect) equal(other objectImpl) bool {
	if other, ok := other.(*objectGoReflect); ok {
		k1, k2 := o.fieldsValue.Kind(), other.fieldsValue.Kind()
		if k1 == k2 {
			if isContainer(k1) {
				return o.fieldsValue == other.fieldsValue
			}
			return o.fieldsValue.Interface() == other.fieldsValue.Interface()
		}
	}
	return false
}

func (o *objectGoReflect) reflectValue() reflect.Value {
	return o.fieldsValue
}

func (o *objectGoReflect) setReflectValue(v reflect.Value) {
	o.fieldsValue = v
	o.origValue = v
	o.methodsValue = v.Addr()
}

func (o *objectGoReflect) esValue() Value {
	return o.val
}

func (r *Runtime) buildFieldInfo(t reflect.Type, index []int, info *reflectFieldsInfo) {
	n := t.NumField()
	for i := 0; i < n; i++ {
		field := t.Field(i)
		name := field.Name
		if !ast.IsExported(name) {
			continue
		}
		if r.fieldNameMapper != nil {
			name = r.fieldNameMapper.FieldName(t, field)
		}

		if name != "" {
			if inf, exists := info.Fields[name]; !exists {
				info.Names = append(info.Names, name)
			} else {
				if len(inf.Index) <= len(index) {
					continue
				}
			}
		}

		if name != "" || field.Anonymous {
			idx := make([]int, len(index)+1)
			copy(idx, index)
			idx[len(idx)-1] = i

			if name != "" {
				info.Fields[name] = reflectFieldInfo{
					Index:     idx,
					Anonymous: field.Anonymous,
				}
			}
			if field.Anonymous {
				typ := field.Type
				for typ.Kind() == reflect.Ptr {
					typ = typ.Elem()
				}
				if typ.Kind() == reflect.Struct {
					r.buildFieldInfo(typ, idx, info)
				}
			}
		}
	}
}

var emptyMethodsInfo = reflectMethodsInfo{}

func (r *Runtime) buildMethodsInfo(t reflect.Type) (info *reflectMethodsInfo) {
	n := t.NumMethod()
	if n == 0 {
		return &emptyMethodsInfo
	}
	info = new(reflectMethodsInfo)
	info.Methods = make(map[string]int, n)
	info.Names = make([]string, 0, n)
	for i := 0; i < n; i++ {
		method := t.Method(i)
		name := method.Name
		if !ast.IsExported(name) {
			continue
		}
		if r.fieldNameMapper != nil {
			name = r.fieldNameMapper.MethodName(t, method)
			if name == "" {
				continue
			}
		}

		if _, exists := info.Methods[name]; !exists {
			info.Names = append(info.Names, name)
		}

		info.Methods[name] = i
	}
	return
}

func (r *Runtime) buildFieldsInfo(t reflect.Type) (info *reflectFieldsInfo) {
	info = new(reflectFieldsInfo)
	n := t.NumField()
	info.Fields = make(map[string]reflectFieldInfo, n)
	info.Names = make([]string, 0, n)
	r.buildFieldInfo(t, nil, info)
	return
}

func (r *Runtime) fieldsInfo(t reflect.Type) (info *reflectFieldsInfo) {
	var exists bool
	if info, exists = r.fieldsInfoCache[t]; !exists {
		info = r.buildFieldsInfo(t)
		if r.fieldsInfoCache == nil {
			r.fieldsInfoCache = make(map[reflect.Type]*reflectFieldsInfo)
		}
		r.fieldsInfoCache[t] = info
	}

	return
}

func (r *Runtime) methodsInfo(t reflect.Type) (info *reflectMethodsInfo) {
	var exists bool
	if info, exists = r.methodsInfoCache[t]; !exists {
		info = r.buildMethodsInfo(t)
		if r.methodsInfoCache == nil {
			r.methodsInfoCache = make(map[reflect.Type]*reflectMethodsInfo)
		}
		r.methodsInfoCache[t] = info
	}

	return
}

// SetFieldNameMapper sets a custom field name mapper for Go types. It can be called at any time, however
// the mapping for any given value is fixed at the point of creation.
// Setting this to nil restores the default behaviour which is all exported fields and methods are mapped to their
// original unchanged names.
func (r *Runtime) SetFieldNameMapper(mapper FieldNameMapper) {
	r.fieldNameMapper = mapper
	r.fieldsInfoCache = nil
	r.methodsInfoCache = nil
}

// TagFieldNameMapper returns a FieldNameMapper that uses the given tagName for struct fields and optionally
// uncapitalises (making the first letter lower case) method names.
// The common tag value syntax is supported (name[,options]), however options are ignored.
// Setting name to anything other than a valid ECMAScript identifier makes the field hidden.
func TagFieldNameMapper(tagName string, uncapMethods bool) FieldNameMapper {
	return tagFieldNameMapper{
		tagName:      tagName,
		uncapMethods: uncapMethods,
	}
}

// UncapFieldNameMapper returns a FieldNameMapper that uncapitalises struct field and method names
// making the first letter lower case.
func UncapFieldNameMapper() FieldNameMapper {
	return uncapFieldNameMapper{}
}
