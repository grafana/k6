# Sobek

ECMAScript engine in pure Go, used as k6's JavaScript runtime. Fork of goja.

## Architecture

Three-stage pipeline: **parse -> compile -> execute**. Source text becomes an AST, the compiler transforms the AST into bytecode stored in a compiled program, and a register-based VM executes the bytecode within a runtime instance.

The runtime is the central type. It owns the global object, all JS built-ins, the VM, and a job queue for promise microtasks. **One runtime per goroutine.** It is not goroutine-safe. Objects cannot be passed between runtimes; doing so panics with a type error.

Go values cross the boundary in two directions. Go-to-JS conversion auto-wraps structs, slices, maps, and function signatures into proxy objects. A field name mapper interface controls how Go struct fields and methods appear as JS property names. JS-to-Go export reverses this, returning plain Go types. Primitive values are goroutine-safe and transferable; objects are not.

ESM module support exists but is experimental. It requires the embedder to provide an event loop -- sobek has none. k6 builds its own event loop on top.

Strings use a custom internal representation to handle UTF-16 semantics on top of Go's UTF-8 strings. Conversion between the two is lossy for lone surrogates.

Regex patterns fall back to a third-party engine when Go's stdlib regex cannot handle the pattern (lookbehind, backreferences, etc.).

The promise job queue drains synchronously when the top-level script function returns. On interrupt, the queue is discarded without running pending jobs.

## Gotchas

- **Merging upstream goja**: Sobek periodically merges from the upstream fork. Always use merge commits, never rebase or squash. The upstream remote is conventionally named `goja`.

- **WeakMap values leak**: Values stay reachable as long as the key is reachable, even after the WeakMap is collected. This is a Go GC limitation. WeakRef and FinalizationRegistry cannot be implemented.

- **Broken surrogate pairs in JSON**: Go's stdlib JSON operates on UTF-8, so lone surrogates in JSON strings get replaced with the Unicode replacement character instead of being preserved.

- **No event loop**: There is no setTimeout, setInterval, or any async scheduling. The embedder must provide all concurrency primitives.

- **Interrupt vs. cancel**: Runaway scripts are stopped with the runtime's interrupt method, not context cancellation. After interrupting, the interrupt flag must be explicitly cleared before reuse, or the next execution immediately aborts.

- **Object cross-runtime panic**: Passing an Object created in one runtime to another runtime's method silently compiles but panics at runtime. The check is in the Go-to-JS value conversion path.
