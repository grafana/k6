package wasi_snapshot_preview1

import (
	"context"

	"github.com/tetratelabs/wazero/api"
	. "github.com/tetratelabs/wazero/internal/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/internal/wasm"
)

// clockResGet is the WASI function named ClockResGetName that returns the
// resolution of time values returned by clockTimeGet.
//
// # Parameters
//
//   - id: clock ID to use
//   - resultResolution: offset to write the resolution to api.Memory
//   - the resolution is an uint64 little-endian encoding
//
// Result (Errno)
//
// The return value is ErrnoSuccess except the following error conditions:
//   - ErrnoNotsup: the clock ID is not supported.
//   - ErrnoInval: the clock ID is invalid.
//   - ErrnoFault: there is not enough memory to write results
//
// For example, if the resolution is 100ns, this function writes the below to
// api.Memory:
//
//	                                   uint64le
//	                   +-------------------------------------+
//	                   |                                     |
//	         []byte{?, 0x64, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, ?}
//	resultResolution --^
//
// Note: This is similar to `clock_getres` in POSIX.
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-clock_res_getid-clockid---errno-timestamp
// See https://linux.die.net/man/3/clock_getres
var clockResGet = newHostFunc(ClockResGetName, clockResGetFn, []api.ValueType{i32, i32}, "id", "result.resolution")

func clockResGetFn(_ context.Context, mod api.Module, params []uint64) Errno {
	sysCtx := mod.(*wasm.CallContext).Sys
	id, resultResolution := uint32(params[0]), uint32(params[1])

	var resolution uint64 // ns
	switch id {
	case ClockIDRealtime:
		resolution = uint64(sysCtx.WalltimeResolution())
	case ClockIDMonotonic:
		resolution = uint64(sysCtx.NanotimeResolution())
	default:
		return ErrnoInval
	}

	if !mod.Memory().WriteUint64Le(resultResolution, resolution) {
		return ErrnoFault
	}
	return ErrnoSuccess
}

// clockTimeGet is the WASI function named ClockTimeGetName that returns
// the time value of a name (time.Now).
//
// # Parameters
//
//   - id: clock ID to use
//   - precision: maximum lag (exclusive) that the returned time value may have,
//     compared to its actual value
//   - resultTimestamp: offset to write the timestamp to api.Memory
//   - the timestamp is epoch nanos encoded as a little-endian uint64
//
// Result (Errno)
//
// The return value is ErrnoSuccess except the following error conditions:
//   - ErrnoNotsup: the clock ID is not supported.
//   - ErrnoInval: the clock ID is invalid.
//   - ErrnoFault: there is not enough memory to write results
//
// For example, if time.Now returned exactly midnight UTC 2022-01-01
// (1640995200000000000), and parameters resultTimestamp=1, this function
// writes the below to api.Memory:
//
//	                                    uint64le
//	                  +------------------------------------------+
//	                  |                                          |
//	        []byte{?, 0x0, 0x0, 0x1f, 0xa6, 0x70, 0xfc, 0xc5, 0x16, ?}
//	resultTimestamp --^
//
// Note: This is similar to `clock_gettime` in POSIX.
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-clock_time_getid-clockid-precision-timestamp---errno-timestamp
// See https://linux.die.net/man/3/clock_gettime
var clockTimeGet = newHostFunc(ClockTimeGetName, clockTimeGetFn, []api.ValueType{i32, i64, i32}, "id", "precision", "result.timestamp")

func clockTimeGetFn(_ context.Context, mod api.Module, params []uint64) Errno {
	sysCtx := mod.(*wasm.CallContext).Sys
	id := uint32(params[0])
	// TODO: precision is currently ignored.
	// precision = params[1]
	resultTimestamp := uint32(params[2])

	var val int64
	switch id {
	case ClockIDRealtime:
		val = sysCtx.WalltimeNanos()
	case ClockIDMonotonic:
		val = sysCtx.Nanotime()
	default:
		return ErrnoInval
	}

	if !mod.Memory().WriteUint64Le(resultTimestamp, uint64(val)) {
		return ErrnoFault
	}
	return ErrnoSuccess
}
