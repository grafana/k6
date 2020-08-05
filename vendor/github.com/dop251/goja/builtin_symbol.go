package goja

import "github.com/dop251/goja/unistring"

var (
	symHasInstance        = newSymbol(asciiString("Symbol.hasInstance"))
	symIsConcatSpreadable = newSymbol(asciiString("Symbol.isConcatSpreadable"))
	symIterator           = newSymbol(asciiString("Symbol.iterator"))
	symMatch              = newSymbol(asciiString("Symbol.match"))
	symReplace            = newSymbol(asciiString("Symbol.replace"))
	symSearch             = newSymbol(asciiString("Symbol.search"))
	symSpecies            = newSymbol(asciiString("Symbol.species"))
	symSplit              = newSymbol(asciiString("Symbol.split"))
	symToPrimitive        = newSymbol(asciiString("Symbol.toPrimitive"))
	symToStringTag        = newSymbol(asciiString("Symbol.toStringTag"))
	symUnscopables        = newSymbol(asciiString("Symbol.unscopables"))
)

func (r *Runtime) builtin_symbol(call FunctionCall) Value {
	var desc valueString
	if arg := call.Argument(0); !IsUndefined(arg) {
		desc = arg.toString()
	} else {
		desc = stringEmpty
	}
	return newSymbol(desc)
}

func (r *Runtime) symbolproto_tostring(call FunctionCall) Value {
	sym, ok := call.This.(*valueSymbol)
	if !ok {
		if obj, ok := call.This.(*Object); ok {
			if v, ok := obj.self.(*primitiveValueObject); ok {
				if sym1, ok := v.pValue.(*valueSymbol); ok {
					sym = sym1
				}
			}
		}
	}
	if sym == nil {
		panic(r.NewTypeError("Method Symbol.prototype.toString is called on incompatible receiver"))
	}
	return sym.desc
}

func (r *Runtime) symbolproto_valueOf(call FunctionCall) Value {
	_, ok := call.This.(*valueSymbol)
	if ok {
		return call.This
	}

	if obj, ok := call.This.(*Object); ok {
		if v, ok := obj.self.(*primitiveValueObject); ok {
			if sym, ok := v.pValue.(*valueSymbol); ok {
				return sym
			}
		}
	}

	panic(r.NewTypeError("Symbol.prototype.valueOf requires that 'this' be a Symbol"))
}

func (r *Runtime) symbol_for(call FunctionCall) Value {
	key := call.Argument(0).toString()
	keyStr := key.string()
	if v := r.symbolRegistry[keyStr]; v != nil {
		return v
	}
	if r.symbolRegistry == nil {
		r.symbolRegistry = make(map[unistring.String]*valueSymbol)
	}
	v := newSymbol(key)
	r.symbolRegistry[keyStr] = v
	return v
}

func (r *Runtime) symbol_keyfor(call FunctionCall) Value {
	arg := call.Argument(0)
	sym, ok := arg.(*valueSymbol)
	if !ok {
		panic(r.NewTypeError("%s is not a symbol", arg.String()))
	}
	for key, s := range r.symbolRegistry {
		if s == sym {
			return stringValueFromRaw(key)
		}
	}
	return _undefined
}

func (r *Runtime) createSymbolProto(val *Object) objectImpl {
	o := &baseObject{
		class:      classObject,
		val:        val,
		extensible: true,
		prototype:  r.global.ObjectPrototype,
	}
	o.init()

	o._putProp("constructor", r.global.Symbol, true, false, true)
	o._putProp("toString", r.newNativeFunc(r.symbolproto_tostring, nil, "toString", nil, 0), true, false, true)
	o._putProp("valueOf", r.newNativeFunc(r.symbolproto_valueOf, nil, "valueOf", nil, 0), true, false, true)
	o._putSym(symToPrimitive, valueProp(r.newNativeFunc(r.symbolproto_valueOf, nil, "[Symbol.toPrimitive]", nil, 1), false, false, true))
	o._putSym(symToStringTag, valueProp(newStringValue("Symbol"), false, false, true))

	return o
}

func (r *Runtime) createSymbol(val *Object) objectImpl {
	o := r.newNativeFuncObj(val, r.builtin_symbol, nil, "Symbol", r.global.SymbolPrototype, 0)

	o._putProp("for", r.newNativeFunc(r.symbol_for, nil, "for", nil, 1), true, false, true)
	o._putProp("keyFor", r.newNativeFunc(r.symbol_keyfor, nil, "keyFor", nil, 1), true, false, true)

	for _, s := range []*valueSymbol{
		symHasInstance,
		symIsConcatSpreadable,
		symIterator,
		symMatch,
		symReplace,
		symSearch,
		symSpecies,
		symSplit,
		symToPrimitive,
		symToStringTag,
		symUnscopables,
	} {
		n := s.desc.(asciiString)
		n = n[len("Symbol(Symbol.") : len(n)-1]
		o._putProp(unistring.String(n), s, false, false, false)
	}

	return o
}

func (r *Runtime) initSymbol() {
	r.global.SymbolPrototype = r.newLazyObject(r.createSymbolProto)

	r.global.Symbol = r.newLazyObject(r.createSymbol)
	r.addToGlobal("Symbol", r.global.Symbol)

}
