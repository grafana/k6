goja
====

ECMAScript 5.1(+) implementation in Go.

[![GoDoc](https://godoc.org/github.com/dop251/goja?status.svg)](https://godoc.org/github.com/dop251/goja)

Goja is an implementation of ECMAScript 5.1 in pure Go with emphasis on standard compliance and
performance.

This project was largely inspired by [otto](https://github.com/robertkrimen/otto).

Minimum required Go version is 1.14.

Features
--------

 * Full ECMAScript 5.1 support (including regex and strict mode).
 * Passes nearly all [tc39 tests](https://github.com/tc39/test262) tagged with es5id. The goal is to pass all of them.
   Note, the current working commit is https://github.com/tc39/test262/commit/ddfe24afe3043388827aa220ef623b8540958bbd.
   The next commit removed most of the es5id tags which made it impossible to distinguish which tests to run.
 * Capable of running Babel, Typescript compiler and pretty much anything written in ES5.
 * Sourcemaps.
 * Some ES6 functionality, still work in progress, see https://github.com/dop251/goja/milestone/1?closed=1
 
Known incompatibilities and caveats
-----------------------------------

### WeakMap
WeakMap maintains "hard" references to its values. This means if a value references a key in a WeakMap or a WeakMap
itself, it will not be garbage-collected until the WeakMap becomes unreferenced. To illustrate this:

```go
var m = new WeakMap();
var key = {};
m.set(key, {key: key});
// or m.set(key, key);
key = undefined; // The value will NOT become garbage-collectable at this point
m = undefined; // But it will at this point.
```

Note, this does not have any effect on the application logic, but causes a higher-than-expected memory usage.

FAQ
---

### How fast is it?

Although it's faster than many scripting language implementations in Go I have seen 
(for example it's 6-7 times faster than otto on average) it is not a
replacement for V8 or SpiderMonkey or any other general-purpose JavaScript engine.
You can find some benchmarks [here](https://github.com/dop251/goja/issues/2).

### Why would I want to use it over a V8 wrapper?

It greatly depends on your usage scenario. If most of the work is done in javascript
(for example crypto or any other heavy calculations) you are definitely better off with V8.

If you need a scripting language that drives an engine written in Go so that
you need to make frequent calls between Go and javascript passing complex data structures
then the cgo overhead may outweigh the benefits of having a faster javascript engine.

Because it's written in pure Go there are no cgo dependencies, it's very easy to build and it
should run on any platform supported by Go.

It gives you a much better control over execution environment so can be useful for research.

### Is it goroutine-safe?

No. An instance of goja.Runtime can only be used by a single goroutine
at a time. You can create as many instances of Runtime as you like but 
it's not possible to pass object values between runtimes.

### Where is setTimeout()?

setTimeout() assumes concurrent execution of code which requires an execution
environment, for example an event loop similar to nodejs or a browser.
There is a [separate project](https://github.com/dop251/goja_nodejs) aimed at providing some NodeJS functionality,
and it includes an event loop.

### Can you implement (feature X from ES6 or higher)?

Some ES6 functionality has been implemented. So far this is mostly built-ins, not syntax enhancements.
See https://github.com/dop251/goja/milestone/1 for more details.

The ongoing work is done in the es6 branch which is merged into master when appropriate. Every commit
in this branch represents a relatively stable state (i.e. it compiles and passes all enabled tc39 tests),
however because the version of tc39 tests I use is quite old, it may be not as well tested as the ES5.1
functionality. Because ES6 is a superset of ES5.1 it should not break your existing code.

I will be adding features in their dependency order and as quickly as my time allows. Please do not ask
for ETAs. Features that are open in the [milestone](https://github.com/dop251/goja/milestone/1) are either in progress
or will be worked on next.

### How do I contribute?

Before submitting a pull request please make sure that:

- You followed ECMA standard as close as possible. If adding a new feature make sure you've read the specification,
do not just base it on a couple of examples that work fine.
- Your change does not have a significant negative impact on performance (unless it's a bugfix and it's unavoidable)
- It passes all relevant tc39 tests.

Current Status
--------------

 * There should be no breaking changes in the API, however it may be extended.
 * Some of the AnnexB functionality is missing.

Basic Example
-------------

Run JavaScript and get the result value.

```go
vm := goja.New()
v, err := vm.RunString("2 + 2")
if err != nil {
    panic(err)
}
if num := v.Export().(int64); num != 4 {
    panic(num)
}
```

Passing Values to JS
--------------------
Any Go value can be passed to JS using Runtime.ToValue() method. See the method's [documentation](https://godoc.org/github.com/dop251/goja#Runtime.ToValue) for more details.

Exporting Values from JS
------------------------
A JS value can be exported into its default Go representation using Value.Export() method.

Alternatively it can be exported into a specific Go variable using [Runtime.ExportTo()](https://godoc.org/github.com/dop251/goja#Runtime.ExportTo) method.

Within a single export operation the same Object will be represented by the same Go value (either the same map, slice or
a pointer to the same struct). This includes circular objects and makes it possible to export them.

Calling JS functions from Go
----------------------------
There are 2 approaches:

- Using [AssertFunction()](https://godoc.org/github.com/dop251/goja#AssertFunction):
```go
vm := New()
_, err := vm.RunString(`
function sum(a, b) {
    return a+b;
}
`)
if err != nil {
    panic(err)
}
sum, ok := AssertFunction(vm.Get("sum"))
if !ok {
    panic("Not a function")
}

res, err := sum(Undefined(), vm.ToValue(40), vm.ToValue(2))
if err != nil {
    panic(err)
}
fmt.Println(res)
// Output: 42
```
- Using [Runtime.ExportTo()](https://godoc.org/github.com/dop251/goja#Runtime.ExportTo):
```go
const SCRIPT = `
function f(param) {
    return +param + 2;
}
`

vm := New()
_, err := vm.RunString(SCRIPT)
if err != nil {
    panic(err)
}

var fn func(string) string
err = vm.ExportTo(vm.Get("f"), &fn)
if err != nil {
    panic(err)
}

fmt.Println(fn("40")) // note, _this_ value in the function will be undefined.
// Output: 42
```

The first one is more low level and allows specifying _this_ value, whereas the second one makes the function look like
a normal Go function.

Mapping struct field and method names
-------------------------------------
By default, the names are passed through as is which means they are capitalised. This does not match
the standard JavaScript naming convention, so if you need to make your JS code look more natural or if you are
dealing with a 3rd party library, you can use a [FieldNameMapper](https://godoc.org/github.com/dop251/goja#FieldNameMapper):

```go
vm := New()
vm.SetFieldNameMapper(TagFieldNameMapper("json", true))
type S struct {
    Field int `json:"field"`
}
vm.Set("s", S{Field: 42})
res, _ := vm.RunString(`s.field`) // without the mapper it would have been s.Field
fmt.Println(res.Export())
// Output: 42
```

There are two standard mappers: [TagFieldNameMapper](https://godoc.org/github.com/dop251/goja#TagFieldNameMapper) and
[UncapFieldNameMapper](https://godoc.org/github.com/dop251/goja#UncapFieldNameMapper), or you can use your own implementation.

Native Constructors
-------------------

In order to implement a constructor function in Go use `func (goja.ConstructorCall) *goja.Object`.
See [Runtime.ToValue()](https://godoc.org/github.com/dop251/goja#Runtime.ToValue) documentation for more details.

Regular Expressions
-------------------

Goja uses the embedded Go regexp library where possible, otherwise it falls back to [regexp2](https://github.com/dlclark/regexp2).

Exceptions
----------

Any exception thrown in JavaScript is returned as an error of type *Exception. It is possible to extract the value thrown
by using the Value() method:

```go
vm := New()
_, err := vm.RunString(`

throw("Test");

`)

if jserr, ok := err.(*Exception); ok {
    if jserr.Value().Export() != "Test" {
        panic("wrong value")
    }
} else {
    panic("wrong type")
}
```

If a native Go function panics with a Value, it is thrown as a Javascript exception (and therefore can be caught):

```go
var vm *Runtime

func Test() {
    panic(vm.ToValue("Error"))
}

vm = New()
vm.Set("Test", Test)
_, err := vm.RunString(`

try {
    Test();
} catch(e) {
    if (e !== "Error") {
        throw e;
    }
}

`)

if err != nil {
    panic(err)
}
```

Interrupting
------------

```go
func TestInterrupt(t *testing.T) {
    const SCRIPT = `
    var i = 0;
    for (;;) {
        i++;
    }
    `

    vm := New()
    time.AfterFunc(200 * time.Millisecond, func() {
        vm.Interrupt("halt")
    })

    _, err := vm.RunString(SCRIPT)
    if err == nil {
        t.Fatal("Err is nil")
    }
    // err is of type *InterruptError and its Value() method returns whatever has been passed to vm.Interrupt()
}
```

NodeJS Compatibility
--------------------

There is a [separate project](https://github.com/dop251/goja_nodejs) aimed at providing some of the NodeJS functionality.
