package wasi_snapshot_preview1

import (
	"context"

	"github.com/tetratelabs/wazero/api"
	. "github.com/tetratelabs/wazero/internal/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/internal/wasm"
)

// schedYield is the WASI function named SchedYieldName which temporarily
// yields execution of the calling thread.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-sched_yield---errno
var schedYield = newHostFunc(SchedYieldName, schedYieldFn, nil)

func schedYieldFn(_ context.Context, mod api.Module, _ []uint64) Errno {
	sysCtx := mod.(*wasm.CallContext).Sys
	sysCtx.Osyield()
	return ErrnoSuccess
}
