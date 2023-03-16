# wazero: the zero dependency WebAssembly runtime for Go developers

[![WebAssembly Core Specification Test](https://github.com/tetratelabs/wazero/actions/workflows/spectest.yaml/badge.svg)](https://github.com/tetratelabs/wazero/actions/workflows/spectest.yaml) [![Go Reference](https://pkg.go.dev/badge/github.com/tetratelabs/wazero.svg)](https://pkg.go.dev/github.com/tetratelabs/wazero) [![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

WebAssembly is a way to safely run code compiled in other languages. Runtimes
execute WebAssembly Modules (Wasm), which are most often binaries with a `.wasm`
extension.

wazero is a WebAssembly Core Specification [1.0][1] and [2.0][2] compliant
runtime written in Go. It has *zero dependencies*, and doesn't rely on CGO.
This means you can run applications in other languages and still keep cross
compilation.

Import wazero and extend your Go application with code written in any language!

## Example

The best way to learn wazero is by trying one of our [examples](examples/README.md). The
most [basic example](examples/basic) extends a Go application with an addition
function defined in WebAssembly.

## Runtime

There are two runtime configurations supported in wazero: _Compiler_ is default:

By default, ex `wazero.NewRuntime(ctx)`, the Compiler is used if supported. You
can also force the interpreter like so:
```go
r := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfigInterpreter())
```

### Interpreter
Interpreter is a naive interpreter-based implementation of Wasm virtual
machine. Its implementation doesn't have any platform (GOARCH, GOOS) specific
code, therefore _interpreter_ can be used for any compilation target available
for Go (such as `riscv64`).

### Compiler
Compiler compiles WebAssembly modules into machine code ahead of time (AOT),
during `Runtime.CompileModule`. This means your WebAssembly functions execute
natively at runtime. Compiler is faster than Interpreter, often by order of
magnitude (10x) or more. This is done without host-specific dependencies.

If interested, check out the [RATIONALE.md][8] and help us optimize further!

### Conformance

Both runtimes pass WebAssembly Core [1.0][7] and [2.0][14] specification tests
on supported platforms:

|   Runtime   |                 Usage                  | amd64 | arm64 | others |
|:-----------:|:--------------------------------------:|:-----:|:-----:|:------:|
| Interpreter | `wazero.NewRuntimeConfigInterpreter()` |   ✅   |   ✅   |   ✅    |
|  Compiler   |  `wazero.NewRuntimeConfigCompiler()`   |   ✅   |   ✅   |   ❌    |

## Support Policy

The below support policy focuses on compatability concerns of those embedding
wazero into their Go applications.

### wazero

wazero is an early project, so APIs are subject to change until version 1.0.
To use wazero meanwhile, you need to use the latest pre-release like this:

```bash
go get github.com/tetratelabs/wazero@latest
```

wazero will tag a new pre-release at least once a month until 1.0. 1.0 is
scheduled for March 2023 and will require minimally Go 1.18. Except
experimental packages, wazero will not break API on subsequent minor versions.

Meanwhile, please practice the current APIs to ensure they work for you, and
give us a [star][15] if you are enjoying it so far!

### Go

wazero has no dependencies except Go, so the only source of conflict in your
project's use of wazero is the Go version.

wazero follows the same version policy as Go's [Release Policy][10]: two
versions. wazero will ensure these versions work and bugs are valid if there's
an issue with a current Go version.

Additionally, wazero intentionally delays usage of language or standard library
features one additional version. For example, when Go 1.29 is released, wazero
can use language features or standard libraries added in 1.27. This is a
convenience for embedders who have a slower version policy than Go. However,
only supported Go versions may be used to raise support issues.

### Platform

wazero has two runtime modes: Interpreter and Compiler. The only supported operating
systems are ones we test, but that doesn't necessarily mean other operating
system versions won't work.

We currently test Linux (Ubuntu and scratch), MacOS and Windows as packaged by
[GitHub Actions][11], as well compilation of 32-bit Linux and 64-bit FreeBSD.

* Interpreter
  * Linux is tested on amd64 (native) as well arm64 and riscv64 via emulation.
  * MacOS and Windows are only tested on amd64.
* Compiler
  * Linux is tested on amd64 (native) as well arm64 via emulation.
  * MacOS and Windows are only tested on amd64.

wazero has no dependencies and doesn't require CGO. This means it can also be
embedded in an application that doesn't use an operating system. This is a main
differentiator between wazero and alternatives.

We verify zero dependencies by running tests in Docker's [scratch image][12].
This approach ensures compatibility with any parent image.

-----
wazero is a registered trademark of Tetrate.io, Inc. in the United States and/or other countries

[1]: https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/
[2]: https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/
[4]: https://github.com/WebAssembly/meetings/blob/main/process/subgroups.md
[5]: https://github.com/WebAssembly/WASI
[6]: https://pkg.go.dev/golang.org/x/sys/unix
[7]: https://github.com/WebAssembly/spec/tree/wg-1.0/test/core
[8]: internal/engine/compiler/RATIONALE.md
[9]: https://github.com/tetratelabs/wazero/issues/506
[10]: https://go.dev/doc/devel/release
[11]: https://github.com/actions/virtual-environments
[12]: https://docs.docker.com/develop/develop-images/baseimages/#create-a-simple-parent-image-using-scratch
[13]: https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md
[14]: https://github.com/WebAssembly/spec/tree/d39195773112a22b245ffbe864bab6d1182ccb06/test/core
[15]: https://github.com/tetratelabs/wazero/stargazers
