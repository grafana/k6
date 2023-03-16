package wasm

import (
	"fmt"

	"github.com/tetratelabs/wazero/api"
)

type mutableGlobal struct {
	g *GlobalInstance
}

// compile-time check to ensure mutableGlobal is a api.Global.
var _ api.Global = &mutableGlobal{}

// Type implements the same method as documented on api.Global.
func (g *mutableGlobal) Type() api.ValueType {
	return g.g.Type.ValType
}

// Get implements the same method as documented on api.Global.
func (g *mutableGlobal) Get() uint64 {
	return g.g.Val
}

// Set implements the same method as documented on api.MutableGlobal.
func (g *mutableGlobal) Set(v uint64) {
	g.g.Val = v
}

// String implements fmt.Stringer
func (g *mutableGlobal) String() string {
	switch g.Type() {
	case ValueTypeI32, ValueTypeI64:
		return fmt.Sprintf("global(%d)", g.Get())
	case ValueTypeF32:
		return fmt.Sprintf("global(%f)", api.DecodeF32(g.Get()))
	case ValueTypeF64:
		return fmt.Sprintf("global(%f)", api.DecodeF64(g.Get()))
	default:
		panic(fmt.Errorf("BUG: unknown value type %X", g.Type()))
	}
}

type globalI32 uint64

// compile-time check to ensure globalI32 is a api.Global
var _ api.Global = globalI32(0)

// Type implements the same method as documented on api.Global.
func (g globalI32) Type() api.ValueType {
	return ValueTypeI32
}

// Get implements the same method as documented on api.Global.
func (g globalI32) Get() uint64 {
	return uint64(g)
}

// String implements fmt.Stringer
func (g globalI32) String() string {
	return fmt.Sprintf("global(%d)", g)
}

type globalI64 uint64

// compile-time check to ensure globalI64 is a api.Global
var _ api.Global = globalI64(0)

// Type implements the same method as documented on api.Global.
func (g globalI64) Type() api.ValueType {
	return ValueTypeI64
}

// Get implements the same method as documented on api.Global.
func (g globalI64) Get() uint64 {
	return uint64(g)
}

// String implements fmt.Stringer
func (g globalI64) String() string {
	return fmt.Sprintf("global(%d)", g)
}

type globalF32 uint64

// compile-time check to ensure globalF32 is a api.Global
var _ api.Global = globalF32(0)

// Type implements the same method as documented on api.Global.
func (g globalF32) Type() api.ValueType {
	return ValueTypeF32
}

// Get implements the same method as documented on api.Global.
func (g globalF32) Get() uint64 {
	return uint64(g)
}

// String implements fmt.Stringer
func (g globalF32) String() string {
	return fmt.Sprintf("global(%f)", api.DecodeF32(g.Get()))
}

type globalF64 uint64

// compile-time check to ensure globalF64 is a api.Global
var _ api.Global = globalF64(0)

// Type implements the same method as documented on api.Global.
func (g globalF64) Type() api.ValueType {
	return ValueTypeF64
}

// Get implements the same method as documented on api.Global.
func (g globalF64) Get() uint64 {
	return uint64(g)
}

// String implements fmt.Stringer
func (g globalF64) String() string {
	return fmt.Sprintf("global(%f)", api.DecodeF64(g.Get()))
}
