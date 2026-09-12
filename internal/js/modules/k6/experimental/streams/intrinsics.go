package streams

import "github.com/grafana/sobek"

type readableStreamIntrinsics struct {
	streamPrototype *sobek.Object
	readerPrototype *sobek.Object
}

// This symbol is held only by Go code. The global property is non-enumerable,
// non-configurable, and non-writable, so JavaScript cannot replace the intrinsic record by
// mutating an exported or global constructor.
var readableStreamIntrinsicsKey = sobek.NewSymbol("")

func readableIntrinsicsForRuntime(rt *sobek.Runtime) *readableStreamIntrinsics {
	value := rt.GlobalObject().GetSymbol(readableStreamIntrinsicsKey)
	if value == nil || sobek.IsUndefined(value) {
		return nil
	}
	intrinsics, _ := value.Export().(*readableStreamIntrinsics)
	return intrinsics
}

func storeReadableIntrinsics(rt *sobek.Runtime, intrinsics *readableStreamIntrinsics) error {
	return rt.GlobalObject().DefineDataPropertySymbol(
		readableStreamIntrinsicsKey,
		rt.ToValue(intrinsics),
		sobek.FLAG_FALSE,
		sobek.FLAG_FALSE,
		sobek.FLAG_FALSE,
	)
}

func useConstructorPrototype(rt *sobek.Runtime, constructor sobek.Value, proto *sobek.Object) error {
	constructorObject := constructor.ToObject(rt)
	if err := constructorObject.Set("prototype", proto); err != nil {
		return err
	}
	return proto.DefineDataProperty(
		"constructor",
		constructor,
		sobek.FLAG_TRUE,
		sobek.FLAG_TRUE,
		sobek.FLAG_FALSE,
	)
}
