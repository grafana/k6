// Package wazeroir is a pkg to compile down the standard Wasm binary to wazero's specific IR (wazeroir).
// The wazeroir is inspired by microwasm format (a.k.a. LightbeamIR), previously used
// in the lightbeam compiler in Wasmtime, though it is not specified and only exists
// in the previous codebase of wasmtime
// e.g. https://github.com/bytecodealliance/wasmtime/blob/v0.29.0/crates/lightbeam/src/microwasm.rs
//
// See RATIONALE.md for detail.
package wazeroir
