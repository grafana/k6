# Notable rationale of wazero

## Zero dependencies

Wazero has zero dependencies to differentiate itself from other runtimes which
have heavy impact usually due to CGO. By avoiding CGO, wazero avoids
prerequisites such as shared libraries or libc, and lets users keep features
like cross compilation.

Avoiding go.mod dependencies reduces interference on Go version support, and
size of a statically compiled binary. However, doing so brings some
responsibility into the project.

Go's native platform support is good: We don't need platform-specific code to
get monotonic time, nor do we need much work to implement certain features
needed by our compiler such as `mmap`. That said, Go does not support all
common operating systems to the same degree. For example, Go 1.18 includes
`Mprotect` on Linux and Darwin, but not FreeBSD.

The general tradeoff the project takes from a zero dependency policy is more
explicit support of platforms (in the compiler runtime), as well a larger and
more technically difficult codebase.

At some point, we may allow extensions to supply their own platform-specific
hooks. Until then, one end user impact/tradeoff is some glitches trying
untested platforms (with the Compiler runtime).

### Why not x/sys

Going beyond Go's SDK limitations can be accomplished with their [x/sys library](https://pkg.go.dev/golang.org/x/sys/unix).
For example, this includes `zsyscall_freebsd_amd64.go` missing from the Go SDK.

However, like all dependencies, x/sys is a source of conflict. For example,
x/sys had to be in order to upgrade to Go 1.18.

If we depended on x/sys, we could get more precise functionality needed for
features such as clocks or more platform support for the compiler runtime.

That said, formally supporting an operating system may still require testing as
even use of x/sys can require platform-specifics. For example, [mmap-go](https://github.com/edsrzf/mmap-go)
uses x/sys, but also mentions limitations, some not surmountable with x/sys
alone.

Regardless, we may at some point introduce a separate go.mod for users to use
x/sys as a platform plugin without forcing all users to maintain that
dependency.

## Project structure

wazero uses internal packages extensively to balance API compatability desires for end users with the need to safely
share internals between compilers.

End-user packages include `wazero`, with `Config` structs, `api`, with shared types, and the built-in `wasi` library.
Everything else is internal.

We put the main program for wazero into a directory of the same name to match conventions used in `go install`,
notably the name of the folder becomes the binary name. We chose to use `cmd/wazero` as it is common practice
and less surprising than `wazero/wazero`.

### Internal packages

Most code in wazero is internal, and it is acknowledged that this prevents external implementation of facets such as
compilers or decoding. It also prevents splitting this code into separate repositories, resulting in a larger monorepo.
This also adds work as more code needs to be centrally reviewed.

However, the alternative is neither secure nor viable. To allow external implementation would require exporting symbols
public, such as the `CodeSection`, which can easily create bugs. Moreover, there's a high drift risk for any attempt at
external implementations, compounded not just by wazero's code organization, but also the fast moving Wasm and WASI
specifications.

For example, implementing a compiler correctly requires expertise in Wasm, Golang and assembly. This requires deep
insight into how internals are meant to be structured and the various tiers of testing required for `wazero` to result
in a high quality experience. Even if someone had these skills, supporting external code would introduce variables which
are constants in the central one. Supporting an external codebase is harder on the project team, and could starve time
from the already large burden on the central codebase.

The tradeoffs of internal packages are a larger codebase and responsibility to implement all standard features. It also
implies thinking about extension more as forking is not viable for reasons above also. The primary mitigation of these
realities are friendly OSS licensing, high rigor and a collaborative spirit which aim to make contribution in the shared
codebase productive.

### Avoiding cyclic dependencies

wazero shares constants and interfaces with internal code by a sharing pattern described below:
* shared interfaces and constants go in one package under root: `api`.
* user APIs and structs depend on `api` and go into the root package `wazero`.
  * e.g. `InstantiateModule` -> `/wasm.go` depends on the type `api.Module`.
* implementation code can also depend on `api` in a corresponding package under `/internal`.
  * Ex  package `wasm` -> `/internal/wasm/*.go` and can depend on the type `api.Module`.

The above guarantees no cyclic dependencies at the cost of having to re-define symbols that exist in both packages.
For example, if `wasm.Store` is a type the user needs access to, it is narrowed by a cover type in the `wazero`:

```go
type runtime struct {
	s *wasm.Store
}
```

This is not as bad as it sounds as mutations are only available via configuration. This means exported functions are
limited to only a few functions.

### Avoiding security bugs

In order to avoid security flaws such as code insertion, nothing in the public API is permitted to write directly to any
mutable symbol in the internal package. For example, the package `api` is shared with internal code. To ensure
immutability, the `api` package cannot contain any mutable public symbol, such as a slice or a struct with an exported
field.

In practice, this means shared functionality like memory mutation need to be implemented by interfaces.

Here are some examples:
* `api.Memory` protects access by exposing functions like `WriteFloat64Le` instead of exporting a buffer (`[]byte`).
* There is no exported symbol for the `[]byte` representing the `CodeSection`

Besides security, this practice prevents other bugs and allows centralization of validation logic such as decoding Wasm.

## API Design

### Why is `context.Context` inconsistent?

It may seem strange that only certain API have an initial `context.Context`
parameter. We originally had a `context.Context` for anything that might be
traced, but it turned out to be only useful for lifecycle and host functions.

For instruction-scoped aspects like memory updates, a context parameter is too
fine-grained and also invisible in practice. For example, most users will use
the compiler engine, and its memory, global or table access will never use go's
context.

### Why does `api.ValueType` map to uint64?

WebAssembly allows functions to be defined either by the guest or the host,
with signatures expressed as WebAssembly types. For example, `i32` is a 32-bit
type which might be interpreted as signed. Function signatures can have zero or
more parameters or results even if WebAssembly 1.0 allows up to one result.

The guest can export functions, so that the host can call it. In the case of
wazero, the host is Go and an exported function can be called via
`api.Function`. `api.Function` allows users to supply parameters and read
results as a slice of uint64. For example, if there are no results, an empty
slice is returned. The user can learn the signature via `FunctionDescription`,
which returns the `api.ValueType` corresponding to each parameter or result.
`api.ValueType` defines the mapping of WebAssembly types to `uint64` values for
reason described in this section. The special case of `v128` is also mentioned
below.

wazero maps each value type to a uint64 values because it holds the largest
type in WebAssembly 1.0 (i64). A slice allows you to express empty (e.g. a
nullary signature), for example a start function.

Here's an example of calling a function, noting this syntax works for both a
signature `(param i32 i32) (result i32)` and `(param i64 i64) (result i64)`
```go
x, y := uint64(1), uint64(2)
results, err := mod.ExportedFunction("add").Call(ctx, x, y)
if err != nil {
	log.Panicln(err)
}
fmt.Printf("%d + %d = %d\n", x, y, results[0])
```

WebAssembly does not define an encoding strategy for host defined parameters or
results. This means the encoding rules above are defined by wazero instead. To
address this, we clarified mapping both in `api.ValueType` and added helper
functions like `api.EncodeF64`. This allows users conversions typical in Go
programming, and utilities to avoid ambiguity and edge cases around casting.

Alternatively, we could have defined a byte buffer based approach and a binary
encoding of value types in and out. For example, an empty byte slice would mean
no values, while a non-empty could use a binary encoding for supported values.
This could work, but it is more difficult for the normal case of i32 and i64.
It also shares a struggle with the current approach, which is that value types
were added after WebAssembly 1.0 and not all of them have an encoding. More on
this below.

In summary, wazero chose an approach for signature mapping because there was
none, and the one we chose biases towards simplicity with integers and handles
the rest with documentation and utilities.

#### Post 1.0 value types

Value types added after WebAssembly 1.0 stressed the current model, as some
have no encoding or are larger than 64 bits. While problematic, these value
types are not commonly used in exported (extern) functions. However, some
decisions were made and detailed below.

For example `externref` has no guest representation. wazero chose to map
references to uint64 as that's the largest value needed to encode a pointer on
supported platforms. While there are two reference types, `externref` and
`functype`, the latter is an internal detail of function tables, and the former
is rarely if ever used in function signatures as of the end of 2022.

The only value larger than 64 bits is used for SIMD (`v128`). Vectorizing via
host functions is not used as of the end of 2022. Even if it were, it would be
inefficient vs guest vectorization due to host function overhead. In other
words, the `v128` value type is unlikely to be in an exported function
signature. That it requires two uint64 values to encode is an internal detail
and not worth changing the exported function interface `api.Function`, as doing
so would break all users.

### Interfaces, not structs

All exported types in public packages, regardless of configuration vs runtime, are interfaces. The primary benefits are
internal flexibility and avoiding people accidentally mis-initializing by instantiating the types on their own vs using
the `NewXxx` constructor functions. In other words, there's less support load when things can't be done incorrectly.

Here's an example:
```go
rt := &RuntimeConfig{} // not initialized properly (fields are nil which shouldn't be)
rt := RuntimeConfig{} // not initialized properly (should be a pointer)
rt := wazero.NewRuntimeConfig() // initialized properly
```

There are a few drawbacks to this, notably some work for maintainers.
* Interfaces are decoupled from the structs implementing them, which means the signature has to be repeated twice.
* Interfaces have to be documented and guarded at time of use, that 3rd party implementations aren't supported.
* As of Golang 1.18, interfaces are still [not well supported](https://github.com/golang/go/issues/5860) in godoc.

## Config

wazero configures scopes such as Runtime and Module using `XxxConfig` types. For example, `RuntimeConfig` configures
`Runtime` and `ModuleConfig` configure `Module` (instantiation). In all cases, config types begin defaults and can be
customized by a user, e.g., selecting features or a module name override.

### Why don't we make each configuration setting return an error?
No config types create resources that would need to be closed, nor do they return errors on use. This helps reduce
resource leaks, and makes chaining easier. It makes it possible to parse configuration (ex by parsing yaml) independent
of validating it.

Instead of:
```
cfg, err = cfg.WithFS(fs)
if err != nil {
  return err
}
cfg, err = cfg.WithName(name)
if err != nil {
  return err
}
mod, err = rt.InstantiateModuleWithConfig(ctx, code, cfg)
if err != nil {
  return err
}
```

There's only one call site to handle errors:
```
cfg = cfg.WithFS(fs).WithName(name)
mod, err = rt.InstantiateModuleWithConfig(ctx, code, cfg)
if err != nil {
  return err
}
```

This allows users one place to look for errors, and also the benefit that if anything internally opens a resource, but
errs, there's nothing they need to close. In other words, users don't need to track which resources need closing on
partial error, as that is handled internally by the only code that can read configuration fields.

### Why are configuration immutable?
While it seems certain scopes like `Runtime` won't repeat within a process, they do, possibly in different goroutines.
For example, some users create a new runtime for each module, and some re-use the same base module configuration with
only small updates (ex the name) for each instantiation. Making configuration immutable allows them to be safely used in
any goroutine.

Since config are immutable, changes apply via return val, similar to `append` in a slice.

For example, both of these are the same sort of error:
```go
append(slice, element) // bug as only the return value has the updated slice.
cfg.WithName(next) // bug as only the return value has the updated name.
```

Here's an example of correct use: re-assigning explicitly or via chaining.
```go
cfg = cfg.WithName(name) // explicit

mod, err = rt.InstantiateModuleWithConfig(ctx, code, cfg.WithName(name)) // implicit
if err != nil {
  return err
}
```

### Why aren't configuration assigned with option types?
The option pattern is a familiar one in Go. For example, someone defines a type `func (x X) err` and uses it to update
the target. For example, you could imagine wazero could choose to make `ModuleConfig` from options vs chaining fields.

Ex instead of:
```go
type ModuleConfig interface {
	WithName(string) ModuleConfig
	WithFS(fs.FS) ModuleConfig
}

struct moduleConfig {
	name string
	fs fs.FS
}

func (c *moduleConfig) WithName(name string) ModuleConfig {
    ret := *c // copy
    ret.name = name
    return &ret
}

func (c *moduleConfig) WithFS(fs fs.FS) ModuleConfig {
    ret := *c // copy
    ret.setFS("/", fs)
    return &ret
}

config := r.NewModuleConfig().WithFS(fs)
configDerived := config.WithName("name")
```

An option function could be defined, then refactor each config method into an name prefixed option function:
```go
type ModuleConfig interface {
}
struct moduleConfig {
    name string
    fs fs.FS
}

type ModuleConfigOption func(c *moduleConfig)

func ModuleConfigName(name string) ModuleConfigOption {
    return func(c *moduleConfig) {
        c.name = name
	}
}

func ModuleConfigFS(fs fs.FS) ModuleConfigOption {
    return func(c *moduleConfig) {
        c.fs = fs
    }
}

func (r *runtime) NewModuleConfig(opts ...ModuleConfigOption) ModuleConfig {
	ret := newModuleConfig() // defaults
    for _, opt := range opts {
        opt(&ret.config)
    }
    return ret
}

func (c *moduleConfig) WithOptions(opts ...ModuleConfigOption) ModuleConfig {
    ret := *c // copy base config
    for _, opt := range opts {
        opt(&ret.config)
    }
    return ret
}

config := r.NewModuleConfig(ModuleConfigFS(fs))
configDerived := config.WithOptions(ModuleConfigName("name"))
```

wazero took the path of the former design primarily due to:
* interfaces provide natural namespaces for their methods, which is more direct than functions with name prefixes.
* parsing config into function callbacks is more direct vs parsing config into a slice of functions to do the same.
* in either case derived config is needed and the options pattern is more awkward to achieve that.

There are other reasons such as test and debug being simpler without options: the above list is constrained to conserve
space. It is accepted that the options pattern is common in Go, which is the main reason for documenting this decision.

### Why aren't config types deeply structured?
wazero's configuration types cover the two main scopes of WebAssembly use:
* `RuntimeConfig`: This is the broadest scope, so applies also to compilation
  and instantiation. e.g. This controls the WebAssembly Specification Version.
* `ModuleConfig`: This affects modules instantiated after compilation and what
  resources are allowed. e.g. This defines how or if STDOUT is captured. This
  also allows sub-configuration of `FSConfig`.

These default to a flat definition each, with lazy sub-configuration only after
proven to be necessary. A flat structure is easier to work with and is also
easy to discover. Unlike the option pattern described earlier, more
configuration in the interface doesn't taint the package namespace, only
`ModuleConfig`.

We default to a flat structure to encourage simplicity. If we eagerly broke out
all possible configurations into sub-types (e.g. ClockConfig), it would be hard
to notice configuration sprawl. By keeping the config flat, it is easy to see
the cognitive load we may be adding to our users.

In other words, discomfort adding more configuration is a feature, not a bug.
We should only add new configuration rarely, and before doing so, ensure it
will be used. In fact, this is why we support using context fields for
experimental configuration. By letting users practice, we can find out if a
configuration was a good idea or not before committing to it, and potentially
sprawling our types.

In reflection, this approach worked well for the nearly 1.5 year period leading
to version 1.0. We've only had to create a single sub-configuration, `FSConfig`,
and it was well understood why when it occurred.

## Why does InstantiateModule call "_start" by default?
We formerly had functions like `StartWASICommand` that would verify preconditions and start WASI's "_start" command.
However, this caused confusion because both many languages compiled a WASI dependency, and many did so inconsistently.

That said, if "_start" isn't called, it causes issues in TinyGo, as it needs this in order to implement panic. To deal
with this a different way, we have a configuration to call any start functions that exist, which defaults to "_start".

## Runtime == Engine+Store
wazero defines a single user-type which combines the specification concept of `Store` with the unspecified `Engine`
which manages them.

### Why not multi-store?
Multi-store isn't supported as the extra tier complicates lifecycle and locking. Moreover, in practice it is unusual for
there to be an engine that has multiple stores which have multiple modules. More often, it is the case that there is
either 1 engine with 1 store and multiple modules, or 1 engine with many stores, each having 1 non-host module. In worst
case, a user can use multiple runtimes until "multi-store" is better understood.

If later, we have demand for multiple stores, that can be accomplished by overload. e.g. `Runtime.InstantiateInStore` or
`Runtime.Store(name) Store`.

## wazeroir
wazero's intermediate representation (IR) is called `wazeroir`. Lowering into an IR provides us a faster interpreter
and a closer to assembly representation for used by our compiler.

### Intermediate Representation (IR) design
`wazeroir`'s initial design borrowed heavily from the defunct `microwasm` format (a.k.a. LightbeamIR). Notably,
`wazeroir` doesn't have block operations: this simplifies the implementation.

Note: `microwasm` was never specified formally, and only exists in a historical codebase of wasmtime:
https://github.com/bytecodealliance/wasmtime/blob/v0.29.0/crates/lightbeam/src/microwasm.rs

## Exit

### Why do we return a `sys.ExitError` on exit code zero?

It may be surprising to find an error returned on success (exit code zero).
This can be explained easier when you think of function returns: When results
aren't empty, then you must return an error. This is trickier to explain when
results are empty, such as the case in the "_start" function in WASI.

The main rationale for returning an exit error even if the code is success is
that the module is no longer functional. For example, function exports would
error later. In cases like these, it is better to handle errors where they
occur.

Luckily, it is not common to exit a module during the "_start" function. For
example, the only known compilation target that does this is Emscripten. Most,
such as Rust, TinyGo, or normal wasi-libc, don't. If they did, it would
invalidate their function exports. This means it is unlikely most compilers
will change this behavior.

In summary, we return a `sys.ExitError` to the caller whenever we get it, as it
properly reflects the state of the module, which would be closed on this error.

### Why panic with `sys.ExitError` after a host function exits?

Currently, the only portable way to stop processing code is via panic. For
example, WebAssembly "trap" instructions, such as divide by zero, are
implemented via panic. This ensures code isn't executed after it.

When code reaches the WASI `proc_exit` instruction, we need to stop processing.
Regardless of the exit code, any code invoked after exit would be in an
inconsistent state. This is likely why unreachable instructions are sometimes
inserted after exit: https://github.com/emscripten-core/emscripten/issues/12322

## WASI

Unfortunately, (WASI Snapshot Preview 1)[https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md] is not formally defined enough, and has APIs with ambiguous semantics.
This section describes how Wazero interprets and implements the semantics of several WASI APIs that may be interpreted differently by different wasm runtimes.
Those APIs may affect the portability of a WASI application.

### Why don't we attempt to pass wasi-testsuite on user-defined `fs.FS`?

While most cases work fine on an `os.File` based implementation, we won't
promise wasi-testsuite compatibility on user defined wrappers of `os.DirFS`.
The only option for real systems is to use our `sysfs.FS`.

There are a lot of areas where windows behaves differently, despite the
`os.File` abstraction. This goes well beyond file locking concerns (e.g.
`EBUSY` errors on open files). For example, errors like `ACCESS_DENIED` aren't
properly mapped to `EPERM`. There are trickier parts too. `FileInfo.Sys()`
doesn't return enough information to build inodes needed for WASI. To rebuild
them requires the full path to the underlying file, not just its directory
name, and there's no way for us to get that information. At one point we tried,
but in practice things became tangled and functionality such as read-only
wrappers became untenable. Finally, there are version-specific behaviors which
are difficult to maintain even in our own code. For example, go 1.20 opens
files in a different way than versions before it.

### Why aren't WASI rules enforced?

The [snapshot-01](https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md) version of WASI has a
number of rules for a "command module", but only the memory export rule is enforced. If a "_start" function exists, it
is enforced to be the correct signature and succeed, but the export itself isn't enforced. It follows that this means
exports are not required to be contained to a "_start" function invocation. Finally, the "__indirect_function_table"
export is also not enforced.

The reason for the exceptions are that implementations aren't following the rules. For example, TinyGo doesn't export
"__indirect_function_table", so crashing on this would make wazero unable to run TinyGo modules. Similarly, modules
loaded by wapc-go don't always define a "_start" function. Since "snapshot-01" is not a proper version, and certainly
not a W3C recommendation, there's no sense in breaking users over matters like this.

### Why is I/O configuration not coupled to WASI?

WebAssembly System Interfaces (WASI) is a formalization of a practice that can be done anyway: Define a host function to
access a system interface, such as writing to STDOUT. WASI stalled at snapshot-01 and as of early 2023, is being
rewritten entirely.

This instability implies a need to transition between WASI specs, which places wazero in a position that requires
decoupling. For example, if code uses two different functions to call `fd_write`, the underlying configuration must be
centralized and decoupled. Otherwise, calls using the same file descriptor number will end up writing to different
places.

In short, wazero defined system configuration in `ModuleConfig`, not a WASI type. This allows end-users to switch from
one spec to another with minimal impact. This has other helpful benefits, as centralized resources are simpler to close
coherently (ex via `Module.Close`).

In reflection, this worked well as more ABI became usable in wazero. For example, `GOARCH=wasm GOOS=js` code uses the
same `ModuleConfig` (and `FSConfig`) WASI uses, and in compatible ways.

### Background on `ModuleConfig` design

WebAssembly 1.0 (20191205) specifies some aspects to control isolation between modules ([sandboxing](https://en.wikipedia.org/wiki/Sandbox_(computer_security))).
For example, `wasm.Memory` has size constraints and each instance of it is isolated from each other. While `wasm.Memory`
can be shared, by exporting it, it is not exported by default. In fact a WebAssembly Module (Wasm) has no memory by
default.

While memory is defined in WebAssembly 1.0 (20191205), many aspects are not. Let's use an example of `exec.Cmd` as for
example, a WebAssembly System Interfaces (WASI) command is implemented as a module with a `_start` function, and in many
ways acts similar to a process with a `main` function.

To capture "hello world" written to the console (stdout a.k.a. file descriptor 1) in `exec.Cmd`, you would set the
`Stdout` field accordingly, perhaps to a buffer. In WebAssembly 1.0 (20191205), the only way to perform something like
this is via a host function (ex `HostModuleFunctionBuilder`) and internally copy memory corresponding to that string
to a buffer.

WASI implements system interfaces with host functions. Concretely, to write to console, a WASI command `Module` imports
"fd_write" from "wasi_snapshot_preview1" and calls it with the `fd` parameter set to 1 (STDOUT).

The [snapshot-01](https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md) version of WASI has no
means to declare configuration, although its function definitions imply configuration for example if fd 1 should exist,
and if so where should it write. Moreover, snapshot-01 was last updated in late 2020 and the specification is being
completely rewritten as of early 2022. This means WASI as defined by "snapshot-01" will not clarify aspects like which
file descriptors are required. While it is possible a subsequent version may, it is too early to tell as no version of
WASI has reached a stage near W3C recommendation. Even if it did, module authors are not required to only use WASI to
write to console, as they can define their own host functions, such as they did before WASI existed.

wazero aims to serve Go developers as a primary function, and help them transition between WASI specifications. In
order to do this, we have to allow top-level configuration. To ensure isolation by default, `ModuleConfig` has WithXXX
that override defaults to no-op or empty. One `ModuleConfig` instance is used regardless of how many times the same WASI
functions are imported. The nil defaults allow safe concurrency in these situations, as well lower the cost when they
are never used. Finally, a one-to-one mapping with `Module` allows the module to close the `ModuleConfig` instead of
confusing users with another API to close.

Naming, defaults and validation rules of aspects like `STDIN` and `Environ` are intentionally similar to other Go
libraries such as `exec.Cmd` or `syscall.SetEnv`, and differences called out where helpful. For example, there's no goal
to emulate any operating system primitive specific to Windows (such as a 'c:\' drive). Moreover, certain defaults
working with real system calls are neither relevant nor safe to inherit: For example, `exec.Cmd` defaults to read STDIN
from a real file descriptor ("/dev/null"). Defaulting to this, vs reading `io.EOF`, would be unsafe as it can exhaust
file descriptors if resources aren't managed properly. In other words, blind copying of defaults isn't wise as it can
violate isolation or endanger the embedding process. In summary, we try to be similar to normal Go code, but often need
act differently and document `ModuleConfig` is more about emulating, not necessarily performing real system calls.

## File systems

### Why doesn't wazero implement the working directory?

An early design of wazero's API included a `WithWorkDirFS` which allowed
control over which file a relative path such as "./config.yml" resolved to,
independent of the root file system. This intended to help separate concerns
like mutability of files, but it didn't work and was removed.

Compilers that target wasm act differently with regard to the working
directory. For example, while `GOOS=js` uses host functions to track the
working directory, WASI host functions do not. wasi-libc, used by TinyGo,
tracks working directory changes in compiled wasm instead: initially "/" until
code calls `chdir`. Zig assumes the first pre-opened file descriptor is the
working directory.

The only place wazero can standardize a layered concern is via a host function.
Since WASI doesn't use host functions to track the working directory, we can't
standardize the storage and initial value of it.

Meanwhile, code may be able to affect the working directory by compiling
`chdir` into their main function, using an argument or ENV for the initial
value (possibly `PWD`). Those unable to control the compiled code should only
use absolute paths in configuration.

See
* https://github.com/golang/go/blob/go1.20/src/syscall/fs_js.go#L324
* https://github.com/WebAssembly/wasi-libc/pull/214#issue-673090117
* https://github.com/ziglang/zig/blob/53a9ee699a35a3d245ab6d1dac1f0687a4dcb42c/src/main.zig#L32

### Why ignore the error returned by io.Reader when n > 1?

Per https://pkg.go.dev/io#Reader, if we receive an error, any bytes read should
be processed first. At the syscall abstraction (`fd_read`), the caller is the
processor, so we can't process the bytes inline and also return the error (as
`EIO`).

Let's assume we want to return the bytes read on error to the caller. This
implies we at least temporarily ignore the error alongside them. The choice
remaining is whether to persist the error returned with the read until a
possible next call, or ignore the error.

If we persist an error returned, it would be coupled to a file descriptor, but
effectively it is boolean as this case coerces to `EIO`. If we track a "last
error" on a file descriptor, it could be complicated for a couple reasons
including whether the error is transient or permanent, or if the error would
apply to any FD operation, or just read. Finally, there may never be a
subsequent read as perhaps the bytes leading up to the error are enough to
satisfy the processor.

This decision boils down to whether or not to track an error bit per file
descriptor or not. If not, the assumption is that a subsequent operation would
also error, this time without reading any bytes.

The current opinion is to go with the simplest path, which is to return the
bytes read and ignore the error the there were any. Assume a subsequent
operation will err if it needs to. This helps reduce the complexity of the code
in wazero and also accommodates the scenario where the bytes read are enough to
satisfy its processor.

### File descriptor allocation strategy

File descriptor allocation currently uses a strategy similar the one implemented
by unix systems: when opening a file, the lowest unused number is picked.

The WASI standard documents that programs cannot expect that file descriptor
numbers will be allocated with a lowest-first strategy, and they should instead
assume the values will be random. Since _random_ is a very imprecise concept in
computers, we technically satisfying the implementation with the descriptor
allocation strategy we use in Wazero. We could imagine adding more _randomness_
to the descriptor selection process, however this should never be used as a
security measure to prevent applications from guessing the next file number so
there are no strong incentives to complicate the logic.

### Why does `FSConfig.WithDirMount` not match behaviour with `os.DirFS`?

It may seem that we should require any feature that seems like a standard
library in Go, to behave the same way as the standard library. Doing so would
present least surprise to Go developers. In the case of how we handle
filesystems, we break from that as it is incompatible with the expectations of
WASI, the most commonly implemented filesystem ABI.

The main reason is that `os.DirFS` is a virtual filesystem abstraction while
WASI is an abstraction over syscalls. For example, the signature of `fs.Open`
does not permit use of flags. This creates conflict on what default behaviors
to take when Go implemented `os.DirFS`. On the other hand, `path_open` can pass
flags, and in fact tests require them to be honored in specific ways. This
extends beyond WASI as even `GOARCH=wasm GOOS=js` compiled code requires
certain flags passed to `os.OpenFile` which are impossible to pass due to the
signature of `fs.FS`.

This conflict requires us to choose what to be more compatible with, and which
type of user to surprise the least. We assume there will be more developers
compiling code to wasm than developers of custom filesystem plugins, and those
compiling code to wasm will be better served if we are compatible with WASI.
Hence on conflict, we prefer WASI behavior vs the behavior of `os.DirFS`.

Meanwhile, it is possible that Go will one day compile to `GOOS=wasi` in
addition to `GOOS=js`. When there is shared stake in WASI, we expect gaps like
these to be easier to close.

See https://github.com/WebAssembly/wasi-testsuite
See https://github.com/golang/go/issues/58141

### fd_pread: io.Seeker fallback when io.ReaderAt is not supported

`ReadAt` is the Go equivalent to `pread`: it does not affect, and is not
affected by, the underlying file offset. Unfortunately, `io.ReaderAt` is not
implemented by all `fs.File`. For example, as of Go 1.19, `embed.openFile` does
not.

The initial implementation of `fd_pread` instead used `Seek`. To avoid a
regression, we fall back to `io.Seeker` when `io.ReaderAt` is not supported.

This requires obtaining the initial file offset, seeking to the intended read
offset, and resetting the file offset the initial state. If this final seek
fails, the file offset is left in an undefined state. This is not thread-safe.

While seeking per read seems expensive, the common case of `embed.openFile` is
only accessing a single int64 field, which is cheap.

### Pre-opened files

WASI includes `fd_prestat_get` and `fd_prestat_dir_name` functions used to
learn any directory paths for file descriptors open at initialization time.

For example, `__wasilibc_register_preopened_fd` scans any file descriptors past
STDERR (1) and invokes `fd_prestat_dir_name` to learn any path prefixes they
correspond to. Zig's `preopensAlloc` does similar. These pre-open functions are
not used again after initialization.

wazero supports stdio pre-opens followed by any mounts e.g `.:/`. The guest
path is a directory and its name, e.g. "/" is returned by `fd_prestat_dir_name`
for file descriptor 3 (STDERR+1). The first longest match wins on multiple
pre-opens, which allows a path like "/tmp" to match regardless of order vs "/".

See
 * https://github.com/WebAssembly/wasi-libc/blob/a02298043ff551ce1157bc2ee7ab74c3bffe7144/libc-bottom-half/sources/preopens.c
 * https://github.com/ziglang/zig/blob/9cb06f3b8bf9ea6b5e5307711bc97328762d6a1d/lib/std/fs/wasi.zig#L50-L53

### fd_prestat_dir_name

`fd_prestat_dir_name` is a WASI function to return the path of the pre-opened
directory of a file descriptor. It has the following three parameters, and the
third `path_len` has ambiguous semantics.

* `fd`: a file descriptor
* `path`: the offset for the result path
* `path_len`: In wazero, `FdPrestatDirName` writes the result path string to
  `path` offset for the exact length of `path_len`.

Wasmer considers `path_len` to be the maximum length instead of the exact
length  that should be written.
See https://github.com/wasmerio/wasmer/blob/3463c51268ed551933392a4063bd4f8e7498b0f6/lib/wasi/src/syscalls/mod.rs#L764

The semantics in wazero follows that of wasmtime.
See https://github.com/bytecodealliance/wasmtime/blob/2ca01ae9478f199337cf743a6ab543e8c3f3b238/crates/wasi-common/src/snapshots/preview_1.rs#L578-L582

Their semantics match when `path_len` == the length of `path`, so in practice
this difference won't matter match.

## Why does fd_readdir not include dot (".") and dot-dot ("..") entries?

When reading a directory, wazero code does not return dot (".") and dot-dot
("..") entries. The main reason is that Go does not return them from
`os.ReadDir`, and materializing them is complicated (at least dot-dot is).

A directory entry has stat information in it. The stat information includes
inode which is used for comparing file equivalence. In the simple case of dot,
we could materialize a special entry to expose the same info as stat on the fd
would return. However, doing this and not doing dot-dot would cause confusion,
and dot-dot is far more tricky. To back-fill inode information about a parent
directory would be costly and subtle. For example, the pre-open (mount) of the
directory may be different than its logical parent. This is easy to understand
when considering the common case of mounting "/" and "/tmp" as pre-opens. To
implement ".." from "/tmp" requires information from a separate pre-open, this
includes state to even know the difference. There are easier edge cases as
well, such as the decision to not return ".." from a root path. In any case,
this should start to explain that faking entries when underlying stdlib doesn't
return them is tricky and requires quite a lot of state.

Even if we did that, it would cause expense to all users of wazero, so we'd
then look to see if that would be justified or not. However, the most common
compilers involved in end user questions, as of early 2023 are TinyGo, Rust and
Zig. All of these compile code which ignores dot and dot-dot entries. In other
words, faking these entries would not only cost our codebase with complexity,
but it would also add unnecessary overhead as the values aren't commonly used.

The final reason why we might do this, is an end users or a specification
requiring us to. As of early 2023, no end user has raised concern over Go and
by extension wazero not returning dot and dot-dot. The snapshot-01 spec of WASI
does not mention anything on this point. Also, POSIX has the following to say,
which summarizes to "these are optional"

> The readdir() function shall not return directory entries containing empty names. If entries for dot or dot-dot exist, one entry shall be returned for dot and one entry shall be returned for dot-dot; otherwise, they shall not be returned.

In summary, wazero not only doesn't return dot and dot-dot entries because Go
doesn't and emulating them in spite of that would result in no difference
except hire overhead to the majority of our users.

See https://pubs.opengroup.org/onlinepubs/9699919799/functions/readdir.html
See https://github.com/golang/go/blob/go1.20/src/os/dir_unix.go#L108-L110

## sys.Walltime and Nanotime

The `sys` package has two function types, `Walltime` and `Nanotime` for real
and monotonic clock exports. The naming matches conventions used in Go.

```go
func time_now() (sec int64, nsec int32, mono int64) {
	sec, nsec = walltime()
	return sec, nsec, nanotime()
}
```

Splitting functions for wall and clock time allow implementations to choose
whether to implement the clock once (as in Go), or split them out.

Each can be configured with a `ClockResolution`, although is it usually
incorrect as detailed in a sub-heading below. The only reason for exposing this
is to satisfy WASI:

See https://github.com/WebAssembly/wasi-clocks

### Why default to fake time?

WebAssembly has an implicit design pattern of capabilities based security. By
defaulting to a fake time, we reduce the chance of timing attacks, at the cost
of requiring configuration to opt-into real clocks.

See https://gruss.cc/files/fantastictimers.pdf for an example attacks.

### Why does fake time increase on reading?

Both the fake nanotime and walltime increase by 1ms on reading. Particularly in
the case of nanotime, this prevents spinning. For example, when Go compiles
`time.Sleep` using `GOOS=js GOARCH=wasm`, nanotime is used in a loop. If that
never increases, the gouroutine is mistaken for being busy. This would be worse
if a compiler implement sleep using nanotime, yet doesn't check for spinning!

### Why not `time.Clock`?

wazero can't use `time.Clock` as a plugin for clock implementation as it is
only substitutable with build flags (`faketime`) and conflates wall and
monotonic time in the same call.

Go's `time.Clock` was added monotonic time after the fact. For portability with
prior APIs, a decision was made to combine readings into the same API call.

See https://go.googlesource.com/proposal/+/master/design/12914-monotonic.md

WebAssembly time imports do not have the same concern. In fact even Go's
imports for clocks split walltime from nanotime readings.

See https://github.com/golang/go/blob/go1.20/misc/wasm/wasm_exec.js#L243-L255

Finally, Go's clock is not an interface. WebAssembly users who want determinism
or security need to be able to substitute an alternative clock implementation
from the host process one.

### `ClockResolution`

A clock's resolution is hardware and OS dependent so requires a system call to retrieve an accurate value.
Go does not provide a function for getting resolution, so without CGO we don't have an easy way to get an actual
value. For now, we return fixed values of 1us for realtime and 1ns for monotonic, assuming that realtime clocks are
often lower precision than monotonic clocks. In the future, this could be improved by having OS+arch specific assembly
to make syscalls.

For example, Go implements time.Now for linux-amd64 with this [assembly](https://github.com/golang/go/blob/go1.20/src/runtime/time_linux_amd64.s).
Because retrieving resolution is not generally called often, unlike getting time, it could be appropriate to only
implement the fallback logic that does not use VDSO (executing syscalls in user mode). The syscall for clock_getres
is 229 and should be usable. https://pkg.go.dev/syscall#pkg-constants.

If implementing similar for Windows, [mingw](https://github.com/mirror/mingw-w64/blob/6a0e9165008f731bccadfc41a59719cf7c8efc02/mingw-w64-libraries/winpthreads/src/clock.c#L77
) is often a good source to find the Windows API calls that correspond
to a POSIX method.

Writing assembly would allow making syscalls without CGO, but comes with the cost that it will require implementations
across many combinations of OS and architecture.

## sys.Nanosleep

All major programming languages have a `sleep` mechanism to block for a
duration. Sleep is typically implemented by a WASI `poll_oneoff` relative clock
subscription.

For example, the below ends up calling `wasi_snapshot_preview1.poll_oneoff`:

```zig
const std = @import("std");
pub fn main() !void {
    std.time.sleep(std.time.ns_per_s * 5);
}
```

Besides Zig, this is also the case with TinyGo (`-target=wasi`) and Rust
(`--target wasm32-wasi`). This isn't the case with Go (`GOOS=js GOARCH=wasm`),
though. In the latter case, wasm loops on `sys.Nanotime`.

We decided to expose `sys.Nanosleep` to allow overriding the implementation
used in the common case, even if it isn't used by Go, because this gives an
easy and efficient closure over a common program function. We also documented
`sys.Nanotime` to warn users that some compilers don't optimize sleep.

## sys.Osyield

We expose `sys.Osyield`, to allow users to control the behavior of WASI's
`sched_yield` without a new build of wazero. This is mainly for parity with
all other related features which we allow users to implement, including
`sys.Nanosleep`. Unlike others, we don't provide an out-of-box implementation
primarily because it will cause performance problems when accessed.

For example, the below implementation uses CGO, which might result in a 1us
delay per invocation depending on the platform.

See https://github.com/golang/go/issues/19409#issuecomment-284788196
```go
//go:noescape
//go:linkname osyield runtime.osyield
func osyield()
```

In practice, a request to customize this is unlikely to happen until other
thread based functions are implemented. That said, as of early 2023, there are
a few signs of implementation interest and cross-referencing:

See https://github.com/WebAssembly/stack-switching/discussions/38
See https://github.com/WebAssembly/wasi-threads#what-can-be-skipped
See https://slinkydeveloper.com/Kubernetes-controllers-A-New-Hope/

## Signed encoding of integer global constant initializers

wazero treats integer global constant initializers signed as their interpretation is not known at declaration time. For
example, there is no signed integer [value type](https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#value-types%E2%91%A0).

To get at the problem, let's use an example.
```
(global (export "start_epoch") i64 (i64.const 1620216263544))
```

In both signed and unsigned LEB128 encoding, this value is the same bit pattern. The problem is that some numbers are
not. For example, 16256 is `807f` encoded as unsigned, but `80ff00` encoded as signed.

While the specification mentions uninterpreted integers are in abstract [unsigned values](https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#integers%E2%91%A0),
the binary encoding is clear that they are encoded [signed](https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#integers%E2%91%A4).

For consistency, we go with signed encoding in the special case of global constant initializers.

## Implementation limitations

WebAssembly 1.0 (20191205) specification allows runtimes to [limit certain aspects of Wasm module or execution](https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#a2-implementation-limitations).

wazero limitations are imposed pragmatically and described below.

### Number of functions in a module

The possible number of function instances in [a module](https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#module-instances%E2%91%A0) is not specified in the WebAssembly specifications since [`funcaddr`](https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#syntax-funcaddr) corresponding to a function instance in a store can be arbitrary number.
wazero limits the maximum function instances to 2^27 as even that number would occupy 1GB in function pointers.

That is because not only we _believe_ that all use cases are fine with the limitation, but also we have no way to test wazero runtimes under these unusual circumstances.

### Number of function types in a store

There's no limitation on the number of function types in [a store](https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#store%E2%91%A0) according to the spec. In wazero implementation, we assign each function type to a unique ID, and choose to use `uint32` to represent the IDs.
Therefore the maximum number of function types a store can have is limited to 2^27 as even that number would occupy 512MB just to reference the function types.

This is due to the same reason for the limitation on the number of functions above.

### Number of values on the stack in a function

While the the spec does not clarify a limitation of function stack values, wazero limits this to 2^27 = 134,217,728.
The reason is that we internally represent all the values as 64-bit integers regardless of its types (including f32, f64), and 2^27 values means
1 GiB = (2^30). 1 GiB is the reasonable for most applications [as we see a Goroutine has 250 MB as a limit on the stack for 32-bit arch](https://github.com/golang/go/blob/go1.20/src/runtime/proc.go#L152-L159), considering that WebAssembly is (currently) 32-bit environment.

All the functions are statically analyzed at module instantiation phase, and if a function can potentially reach this limit, an error is returned.

### Number of globals in a module

Theoretically, a module can declare globals (including imports) up to 2^32 times. However, wazero limits this to  2^27(134,217,728) per module.
That is because internally we store globals in a slice with pointer types (meaning 8 bytes on 64-bit platforms), and therefore 2^27 globals
means that we have 1 GiB size of slice which seems large enough for most applications.

### Number of tables in a module

While the the spec says that a module can have up to 2^32 tables, wazero limits this to 2^27 = 134,217,728.
One of the reasons is even that number would occupy 1GB in the pointers tables alone. Not only that, we access tables slice by
table index by using 32-bit signed offset in the compiler implementation, which means that the table index of 2^27 can reach 2^27 * 8 (pointer size on 64-bit machines) = 2^30 offsets in bytes.

We _believe_ that all use cases are fine with the limitation, but also note that we have no way to test wazero runtimes under these unusual circumstances.

If a module reaches this limit, an error is returned at the compilation phase.

## Compiler engine implementation

See [compiler/RATIONALE.md](internal/engine/compiler/RATIONALE.md).

## Golang patterns

### Hammer tests
Code that uses concurrency primitives, such as locks or atomics, should include "hammer tests", which run large loops
inside a bounded amount of goroutines, run by half that many `GOMAXPROCS`. These are named consistently "hammer", so
they are easy to find. The name inherits from some existing tests in [golang/go](https://github.com/golang/go/search?q=hammer&type=code).

Here is an annotated description of the key pieces of a hammer test:
1. `P` declares the count of goroutines to use, defaulting to 8 or 4 if `testing.Short`.
   * Half this amount are the cores used, and 4 is less than a modern laptop's CPU. This allows multiple "hammer" tests to run in parallel.
2. `N` declares the scale of work (loop) per goroutine, defaulting to value that finishes in ~0.1s on a modern laptop.
   * When in doubt, try 1000 or 100 if `testing.Short`
   * Remember, there are multiple hammer tests and CI nodes are slow. Slower tests hurt feedback loops.
3. `defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(P/2))` makes goroutines switch cores, testing visibility of shared data.
4. To ensure goroutines execute at the same time, block them with `sync.WaitGroup`, initialized to `Add(P)`.
   * `sync.WaitGroup` internally uses `runtime_Semacquire` not available in any other library.
   * `sync.WaitGroup.Add` with a negative value can unblock many goroutines at the same time, e.g. without a for loop.
5. Track goroutines progress via `finished := make(chan int)` where each goroutine in `P` defers `finished <- 1`.
   1. Tests use `require.XXX`, so `recover()` into `t.Fail` in a `defer` function before `finished <- 1`.
      * This makes it easier to spot larger concurrency problems as you see each failure, not just the first.
   2. After the `defer` function, await unblocked, then run the stateful function `N` times in a normal loop.
      * This loop should trigger shared state problems as locks or atomics are contended by `P` goroutines.
6. After all `P` goroutines launch, atomically release all of them with `WaitGroup.Add(-P)`.
7. Block the runner on goroutine completion, by (`<-finished`) for each `P`.
8. When all goroutines complete, `return` if `t.Failed()`, otherwise perform follow-up state checks.

This is implemented in wazero in [hammer.go](internal/testing/hammer/hammer.go)

### Lock-free, cross-goroutine observations of updates

How to achieve cross-goroutine reads of a variable are not explicitly defined in https://go.dev/ref/mem. wazero uses
atomics to implement this following unofficial practice. For example, a `Close` operation can be guarded to happen only
once via compare-and-swap (CAS) against a zero value. When we use this pattern, we consistently use atomics to both
read and update the same numeric field.

In lieu of formal documentation, we infer this pattern works from other sources (besides tests):
 * `sync.WaitGroup` by definition must support calling `Add` from other goroutines. Internally, it uses atomics.
 * rsc in golang/go#5045 writes "atomics guarantee sequential consistency among the atomic variables".

See https://github.com/golang/go/blob/go1.20/src/sync/waitgroup.go#L64
See https://github.com/golang/go/issues/5045#issuecomment-252730563
See https://www.youtube.com/watch?v=VmrEG-3bWyM
