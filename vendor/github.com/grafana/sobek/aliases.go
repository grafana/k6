package sobek

import (
	"github.com/dop251/goja"
	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/parser"
)

const (
	FLAG_NOT_SET Flag = iota
	FLAG_FALSE
	FLAG_TRUE
)

func Null() Value { return goja.Null() }

func IsInfinity(v Value) bool { return goja.IsInfinity(v) }

func IsNaN(v Value) bool { return goja.IsNaN(v) }

func IsNull(v Value) bool { return goja.IsNull(v) }

func IsUndefined(v Value) bool { return goja.IsUndefined(v) }

func Parse(name, src string, options ...parser.Option) (prg *ast.Program, err error) {
	return goja.Parse(name, src, options...)
}

func AssertFunction(v Value) (Callable, bool) { return goja.AssertFunction(v) }

func AssertConstructor(v Value) (Constructor, bool) { return goja.AssertConstructor(v) }

func Undefined() Value { return goja.Undefined() }

func CompileAST(prg *ast.Program, strict bool) (*Program, error) {
	return goja.CompileAST(prg, strict)
}

func Compile(name, src string, strict bool) (*Program, error) {
	return goja.Compile(name, src, strict)
}

func MustCompile(name, src string, strict bool) *Program {
	return goja.MustCompile(name, src, strict)
}

func New() *Runtime {
	return goja.New()
}

type ArrayBuffer = goja.ArrayBuffer

type (
	AsyncContextTracker = goja.AsyncContextTracker
	Callable            = goja.Callable
)

type (
	CompilerError          = goja.CompilerError
	CompilerReferenceError = goja.CompilerReferenceError
)

type CompilerSyntaxError = goja.CompilerSyntaxError

type Constructor = goja.Constructor

type ConstructorCall = goja.ConstructorCall

type (
	DynamicArray  = goja.DynamicArray
	DynamicObject = goja.DynamicObject
	Exception     = goja.Exception
)

type FieldNameMapper = goja.FieldNameMapper

type (
	Flag         = goja.Flag
	FunctionCall = goja.FunctionCall
)

type InterruptedError = goja.InterruptedError

type (
	JsonEncodable = goja.JsonEncodable
	Now           = goja.Now
	Object        = goja.Object
)

type Program = goja.Program

type Promise = goja.Promise

type (
	PromiseRejectionOperation = goja.PromiseRejectionOperation
	PromiseRejectionTracker   = goja.PromiseRejectionTracker
	PromiseState              = goja.PromiseState
	PropertyDescriptor        = goja.PropertyDescriptor
)

const (
	PromiseRejectionReject PromiseRejectionOperation = iota
	PromiseRejectionHandle
)

const (
	PromiseStatePending PromiseState = iota
	PromiseStateFulfilled
	PromiseStateRejected
)

type Proxy = goja.Proxy

type (
	ProxyTrapConfig = goja.ProxyTrapConfig
	RandSource      = goja.RandSource
	Runtime         = goja.Runtime
)

type StackFrame = goja.StackFrame

type (
	StackOverflowError = goja.StackOverflowError
	String             = goja.String
	StringBuilder      = goja.StringBuilder
	Symbol             = goja.Symbol
	Value              = goja.Value
)

func UncapFieldNameMapper() FieldNameMapper { return goja.UncapFieldNameMapper() }
