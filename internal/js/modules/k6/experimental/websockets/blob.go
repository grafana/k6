package websockets

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"unsafe"

	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/experimental/streams"
	"go.k6.io/k6/js/common"
)

type blob struct {
	typ  string
	data bytes.Buffer
}

func (b *blob) text() string {
	return b.data.String()
}

func (r *WebSocketsAPI) blob(call sobek.ConstructorCall) *sobek.Object {
	rt := r.vu.Runtime()

	b := &blob{}
	var blobParts []interface{}
	if len(call.Arguments) > 0 {
		if err := rt.ExportTo(call.Arguments[0], &blobParts); err != nil {
			common.Throw(rt, fmt.Errorf("failed to process [blobParts]: %w", err))
		}
	}

	if len(blobParts) > 0 {
		r.fillData(b, blobParts, call)
	}

	if len(call.Arguments) > 1 && !sobek.IsUndefined(call.Arguments[1]) {
		opts := call.Arguments[1]
		if !isObject(opts) {
			common.Throw(rt, errors.New("[options] must be an object"))
		}

		typeOpt := opts.ToObject(rt).Get("type")
		if !sobek.IsUndefined(typeOpt) {
			b.typ = typeOpt.String()
		}
	}

	obj := rt.NewObject()
	must(rt, obj.DefineAccessorProperty("size", rt.ToValue(func() sobek.Value {
		return rt.ToValue(b.data.Len())
	}), nil, sobek.FLAG_FALSE, sobek.FLAG_TRUE))
	must(rt, obj.DefineAccessorProperty("type", rt.ToValue(func() sobek.Value {
		return rt.ToValue(b.typ)
	}), nil, sobek.FLAG_FALSE, sobek.FLAG_TRUE))

	must(rt, obj.Set("arrayBuffer", func(_ sobek.FunctionCall) sobek.Value {
		promise, resolve, _ := rt.NewPromise()
		err := resolve(rt.NewArrayBuffer(b.data.Bytes()))
		if err != nil {
			panic(err)
		}
		return rt.ToValue(promise)
	}))
	must(rt, obj.Set("bytes", func(_ sobek.FunctionCall) sobek.Value {
		promise, resolve, reject := rt.NewPromise()
		data, err := rt.New(rt.Get("Uint8Array"), rt.ToValue(b.data.Bytes()))
		if err == nil {
			err = resolve(data)
		} else {
			err = reject(fmt.Errorf("failed to create Uint8Array: %w", err))
		}
		if err != nil {
			panic(err)
		}
		return rt.ToValue(promise)
	}))
	must(rt, obj.Set("slice", func(call sobek.FunctionCall) sobek.Value {
		return r.slice(call, b, rt)
	}))
	must(rt, obj.Set("text", func(_ sobek.FunctionCall) sobek.Value {
		promise, resolve, _ := rt.NewPromise()
		err := resolve(b.text())
		if err != nil {
			panic(err)
		}
		return rt.ToValue(promise)
	}))
	must(rt, obj.Set("stream", func(_ sobek.FunctionCall) sobek.Value {
		return rt.ToValue(streams.NewReadableStreamFromReader(r.vu, &b.data))
	}))

	proto := call.This.Prototype()
	must(rt, proto.Set("toString", func(_ sobek.FunctionCall) sobek.Value {
		return rt.ToValue("[object Blob]")
	}))
	must(rt, obj.SetPrototype(proto))

	return obj
}

func (r *WebSocketsAPI) fillData(b *blob, blobParts []interface{}, call sobek.ConstructorCall) {
	rt := r.vu.Runtime()

	if len(blobParts) > 0 {
		for n, part := range blobParts {
			var err error
			switch v := part.(type) {
			case []uint8:
				_, err = b.data.Write(v)
			case []int8, []int16, []int32, []int64, []uint16, []uint32, []uint64, []float32, []float64:
				_, err = b.data.Write(toByteSlice(v))
			case sobek.ArrayBuffer:
				_, err = b.data.Write(v.Bytes())
			case *sobek.ArrayBuffer:
				_, err = b.data.Write(v.Bytes())
			case string:
				_, err = b.data.WriteString(v)
			case map[string]interface{}:
				obj := call.Arguments[0].ToObject(rt).Get(strconv.FormatInt(int64(n), 10)).ToObject(rt)
				switch {
				case isDataView(obj, rt):
					_, err = b.data.Write(obj.Get("buffer").Export().(sobek.ArrayBuffer).Bytes())
				case isBlob(obj, r.blobConstructor):
					_, err = b.data.Write(extractBytes(obj, rt))
				default:
					err = fmt.Errorf("unsupported type: %T", part)
				}
			default:
				err = fmt.Errorf("unsupported type: %T", part)
			}
			if err != nil {
				common.Throw(rt, fmt.Errorf("failed to process [blobParts]: %w", err))
			}
		}
	}
}

func (r *WebSocketsAPI) slice(call sobek.FunctionCall, b *blob, rt *sobek.Runtime) sobek.Value {
	var (
		from int
		to   = b.data.Len()
		ct   = ""
	)

	if len(call.Arguments) > 0 {
		from = int(call.Arguments[0].ToInteger())
	}

	if len(call.Arguments) > 1 {
		to = int(call.Arguments[1].ToInteger())
		if to < 0 {
			to = b.data.Len() + to
		}
	}

	if len(call.Arguments) > 2 {
		ct = call.Arguments[2].String()
	}

	opts := rt.NewObject()
	must(rt, opts.Set("type", ct))

	sliced, err := rt.New(r.blobConstructor, rt.ToValue([]interface{}{b.data.Bytes()[from:to]}), opts)
	must(rt, err)

	return sliced
}

// toByteSlice converts a slice of numbers to a slice of bytes.
//
//nolint:gosec
func toByteSlice(data interface{}) []byte {
	switch v := data.(type) {
	case []int8:
		return unsafe.Slice((*byte)(unsafe.Pointer(&v[0])), len(v))
	case []uint16:
		return unsafe.Slice((*byte)(unsafe.Pointer(&v[0])), len(v)*2)
	case []int16:
		return unsafe.Slice((*byte)(unsafe.Pointer(&v[0])), len(v)*2)
	case []uint32:
		return unsafe.Slice((*byte)(unsafe.Pointer(&v[0])), len(v)*4)
	case []int32:
		return unsafe.Slice((*byte)(unsafe.Pointer(&v[0])), len(v)*4)
	case []uint64:
		return unsafe.Slice((*byte)(unsafe.Pointer(&v[0])), len(v)*8)
	case []int64:
		return unsafe.Slice((*byte)(unsafe.Pointer(&v[0])), len(v)*8)
	case []float32:
		return unsafe.Slice((*byte)(unsafe.Pointer(&v[0])), len(v)*4)
	case []float64:
		return unsafe.Slice((*byte)(unsafe.Pointer(&v[0])), len(v)*8)
	default:
		// this should never happen
		common.Throw(nil, fmt.Errorf("unsupported type: %T", data))
		return nil
	}
}
