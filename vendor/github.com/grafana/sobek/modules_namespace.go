package sobek

import (
	"sort"

	"github.com/grafana/sobek/unistring"
)

type namespaceObject struct {
	baseObject
	m            ModuleRecord
	exports      map[unistring.String]struct{}
	exportsNames []unistring.String
}

func (r *Runtime) NamespaceObjectFor(m ModuleRecord) *Object {
	if r.moduleNamespaces == nil {
		r.moduleNamespaces = make(map[ModuleRecord]*namespaceObject)
	}
	if o, ok := r.moduleNamespaces[m]; ok {
		return o.val
	}
	o := r.createNamespaceObject(m)
	r.moduleNamespaces[m] = o
	return o.val
}

func (r *Runtime) createNamespaceObject(m ModuleRecord) *namespaceObject {
	o := &Object{runtime: r}
	no := &namespaceObject{m: m}
	no.val = o
	no.extensible = true
	no.defineOwnPropertySym(SymToStringTag, PropertyDescriptor{
		Value: newStringValue("Module"),
	}, true)
	no.extensible = false
	o.self = no
	no.init()
	no.exports = make(map[unistring.String]struct{})
	m.GetExportedNames(func(names []string) {
		for _, exportName := range names {
			v, ambiguous := no.m.ResolveExport(exportName)
			if ambiguous || v == nil {
				continue
			}
			no.exports[unistring.NewFromString(exportName)] = struct{}{}
			no.exportsNames = append(no.exportsNames, unistring.NewFromString(exportName))
		}
	})
	return no
}

func (no *namespaceObject) stringKeys(all bool, accum []Value) []Value {
	for name := range no.exports {
		if !all { //  TODO this seems off
			_ = no.getOwnPropStr(name)
		}
		accum = append(accum, stringValueFromRaw(name))
	}
	// TODO optimize this
	sort.Slice(accum, func(i, j int) bool {
		return accum[i].String() < accum[j].String()
	})
	return accum
}

type namespacePropIter struct {
	no  *namespaceObject
	idx int
}

func (no *namespaceObject) iterateStringKeys() iterNextFunc {
	return (&namespacePropIter{
		no: no,
	}).next
}

func (no *namespaceObject) iterateKeys() iterNextFunc {
	return no.iterateStringKeys()
}

func (i *namespacePropIter) next() (propIterItem, iterNextFunc) {
	for i.idx < len(i.no.exportsNames) {
		name := i.no.exportsNames[i.idx]
		i.idx++
		prop := i.no.getOwnPropStr(name)
		if prop != nil {
			return propIterItem{name: stringValueFromRaw(name), value: prop}, i.next
		}
	}
	return propIterItem{}, nil
}

func (no *namespaceObject) getOwnPropStr(name unistring.String) Value {
	if _, ok := no.exports[name]; !ok {
		return nil
	}
	v, ambiguous := no.m.ResolveExport(name.String())
	if ambiguous || v == nil {
		no.val.runtime.throwReferenceError((name))
	}
	if v.BindingName == "*namespace*" {
		return &valueProperty{
			value:        no.val.runtime.NamespaceObjectFor(v.Module),
			writable:     true,
			configurable: false,
			enumerable:   true,
		}
	}

	mi := no.val.runtime.modules[v.Module]
	b := mi.GetBindingValue(v.BindingName)
	if b == nil {
		// TODO figure this out - this is needed as otherwise valueproperty is thought to not have a value
		// which isn't really correct in a particular test around isFrozen
		b = _null
	}
	return &valueProperty{
		value:        b,
		writable:     true,
		configurable: false,
		enumerable:   true,
	}
}

func (no *namespaceObject) hasOwnPropertyStr(name unistring.String) bool {
	_, ok := no.exports[name]
	return ok
}

func (no *namespaceObject) getStr(name unistring.String, receiver Value) Value {
	prop := no.getOwnPropStr(name)
	if prop, ok := prop.(*valueProperty); ok {
		if receiver == nil {
			return prop.get(no.val)
		}
		return prop.get(receiver)
	}
	return prop
}

func (no *namespaceObject) setOwnStr(name unistring.String, val Value, throw bool) bool {
	no.val.runtime.typeErrorResult(throw, "Cannot add property %s, object is not extensible", name)
	return false
}

func (no *namespaceObject) deleteStr(name unistring.String, throw bool) bool {
	if _, exists := no.exports[name]; exists {
		no.val.runtime.typeErrorResult(throw, "Cannot add property %s, object is not extensible", name)
		return false
	}
	return true
}

func (no *namespaceObject) defineOwnPropertyStr(name unistring.String, desc PropertyDescriptor, throw bool) bool {
	returnFalse := func() bool {
		if throw {
			no.val.runtime.typeErrorResult(throw, "Cannot add property %s, object is not extensible", name)
		}
		return false
	}
	if !no.hasOwnPropertyStr(name) {
		return returnFalse()
	}
	if desc.Empty() {
		return true
	}
	if desc.Writable == FLAG_FALSE {
		return returnFalse()
	}
	if desc.Configurable == FLAG_TRUE {
		return returnFalse()
	}
	if desc.Enumerable == FLAG_FALSE {
		return returnFalse()
	}
	if desc.Value != nil && desc.Value != no.getOwnPropStr(name) {
		return returnFalse()
	}
	return true
}
