package websockets

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/common"
)

// must is a small helper that will panic if err is not nil.
func must(rt *sobek.Runtime, err error) {
	if err != nil {
		common.Throw(rt, err)
	}
}

func isString(o *sobek.Object, rt *sobek.Runtime) bool {
	return o.Prototype().Get("constructor") == rt.GlobalObject().Get("String")
}

func isArray(o *sobek.Object, rt *sobek.Runtime) bool {
	return o.Prototype().Get("constructor") == rt.GlobalObject().Get("Array")
}

func isUint8Array(o *sobek.Object, rt *sobek.Runtime) bool {
	return o.Prototype().Get("constructor") == rt.GlobalObject().Get("Uint8Array")
}

func isDataView(o *sobek.Object, rt *sobek.Runtime) bool {
	return o.Prototype().Get("constructor") == rt.GlobalObject().Get("DataView")
}

func isBlob(o *sobek.Object, blobConstructor sobek.Value) bool {
	return o.Prototype().Get("constructor") == blobConstructor
}

func isObject(val sobek.Value) bool {
	return val != nil && val.ExportType() != nil && val.ExportType().Kind() == reflect.Map
}

func extractBytes(o *sobek.Object, rt *sobek.Runtime) []byte {
	arrayBuffer, ok := sobek.AssertFunction(o.Get("arrayBuffer"))
	if !ok {
		common.Throw(rt, errors.New("Blob.[arrayBuffer] is not a function"))
	}

	buffer, err := arrayBuffer(sobek.Undefined())
	if err != nil {
		common.Throw(rt, fmt.Errorf("call to Blob.[arrayBuffer] failed: %w", err))
	}

	p, ok := buffer.Export().(*sobek.Promise)
	if !ok {
		common.Throw(rt, errors.New("Blob.[arrayBuffer] return is not a Promise"))
	}

	ab, ok := p.Result().Export().(sobek.ArrayBuffer)
	if !ok {
		common.Throw(rt, errors.New("Blob.[arrayBuffer] promise's return is not an ArrayBuffer"))
	}

	return ab.Bytes()
}
