# Compiler engine

This package implements the Compiler engine for WebAssembly *purely written in Go*.
In this README, we describe the background, technical difficulties and some design choices.

## General limitations on pure Go Compiler engines

In Go program, each Goroutine manages its own stack, and each item on Goroutine
stack is managed by Go runtime for garbage collection, etc.

These impose some difficulties on compiler engine purely written in Go because
we *cannot* use native push/pop instructions to save/restore temporary
variables spilling from registers. This results in making it impossible for us
to invoke Go functions from compiled native codes with the native `call`
instruction since it involves stack manipulations.

*TODO: maybe it is possible to hack the runtime to make it possible to achieve
function calls with `call`.*

## How to generate native codes

wazero uses its own assembler, implemented from scratch in the
[`internal/asm`](../../asm/) package. The primary rationale are wazero's zero
dependency policy, and to enable concurrent compilation (a feature the
WebAssembly binary format optimizes for).

Before this, wazero used [`twitchyliquid64/golang-asm`](https://github.com/twitchyliquid64/golang-asm).
However, this was not only a dependency (one of our goals is to have zero
dependencies), but also a large one (several megabytes added to the binary).
Moreover, any copy of golang-asm is not thread-safe, so can't be used for
concurrent compilation (See [#233](https://github.com/tetratelabs/wazero/issues/233)).

The assembled native codes are represented as `[]byte` and the slice region is
marked as executable via mmap system call.

## How to enter native codes

Assuming that we have a native code as `[]byte`, it is straightforward to enter
the native code region via Go assembly code. In this package, we have the
function without body called `nativecall`

```go
func nativecall(codeSegment, engine, memory uintptr)
```

where we pass `codeSegment uintptr` as a first argument. This pointer is to the
first instruction to be executed. The pointer can be easily derived from
`[]byte` via `unsafe.Pointer`:

```go
code := []byte{}
/* ...Compilation ...*/
codeSegment := uintptr(unsafe.Pointer(&code[0]))
nativecall(codeSegment, ...)
```

And `nativecall` is actually implemented in [arch_amd64.s](./arch_amd64.s)
as a convenience layer to comply with the Go's official calling convention.
We delegate the task to jump into the code segment to the Go assembler code.


## Why it's safe to execute runtime-generated machine codes against async Goroutine preemption

Goroutine preemption is the mechanism of the Go runtime to switch goroutines contexts on an OS thread.
There are two types of preemption: cooperative preemption and async preemption. The former happens, for example,
when making a function call, and it is not an issue for our runtime-generated functions as they do not make
direct function calls to Go-implemented functions. On the other hand, the latter, async preemption, can be problematic
since it tries to interrupt the execution of Goroutine at any point of function, and manipulates CPU register states.

Fortunately, our runtime-generated machine codes do not need to take the async preemption into account.
All the assembly codes are entered via the trampoline implemented as Go Assembler Function (e.g. [arch_amd64.s](./arch_amd64.s)),
and as of Go 1.20, these assembler functions are considered as _unsafe_ for async preemption:
- https://github.com/golang/go/blob/go1.20rc1/src/runtime/preempt.go#L406-L407
- https://github.com/golang/go/blob/9f0234214473dfb785a5ad84a8fc62a6a395cbc3/src/runtime/traceback.go#L227

From the Go runtime point of view, the execution of runtime-generated machine codes is considered as a part of
that trampoline function. Therefore, runtime-generated machine code is also correctly considered unsafe for async preemption.

## How to achieve function calls

Given that we cannot use `call` instruction at all in native code, here's how
we achieve the function calls back and forth among Go and (compiled) Wasm
native functions.

TODO:
