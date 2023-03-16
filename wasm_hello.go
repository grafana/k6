//go:build ignore

// Build:
// tinygo build -scheduler=none --no-debug -wasm-abi=generic -target=wasi -o hello.wasm wasm_hello.go
//
// Run:
// k6 run -i=100 -vu=2 hello.wasm

package main

import (
	"fmt"
	"reflect"
	"unsafe"
)

//go:wasm-module env
//export fetch
func fetch(methodptr, methodsz, urlptr, urlsz, bodyptr, bodysz, optptr, optsz uint32) uint32

func ptrToStr(ptr uint32, size uint32) string {
	return *(*string)(unsafe.Pointer(&reflect.SliceHeader{Data: uintptr(ptr), Len: uintptr(size), Cap: uintptr(size)}))
}
func strToPtr(s string) (uint32, uint32) {
	buf := []byte(s)
	ptr := &buf[0]
	unsafePtr := uintptr(unsafe.Pointer(ptr))
	return uint32(unsafePtr), uint32(len(buf))
}

func Fetch(method, url string, body []byte) int {
	mthptr, mthsz := strToPtr(method)
	urlptr, urlsz := strToPtr(url)
	bptr, bsz := uint32(0), uint32(0)
	if body != nil {
		bptr = uint32(uintptr(unsafe.Pointer(&body[0])))
		bsz = uint32(len(body))
	}
	status := fetch(mthptr, mthsz, urlptr, urlsz, bptr, bsz, 0, 0)
	return int(status)
}

//export setup
func Setup() {
	fmt.Println("setup wasm")
}

//export default
func Default() {
	if status := Fetch("GET", "https://zserge.com/", nil); status != 200 {
		panic(status)
	}
}

//export teardown
func TearDown() {
	fmt.Println("teardown")
}

func main() {}
