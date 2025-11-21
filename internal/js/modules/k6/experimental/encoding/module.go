// Package encoding provides a k6 JS module that implements the TextEncoder and
// TextDecoder interfaces.
package encoding

import (
	"errors"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
)

type (
	// RootModule is the global module instance that will create Client
	// instances for each VU.
	RootModule struct{}

	// ModuleInstance represents an instance of the JS module.
	ModuleInstance struct {
		vu modules.VU

		*TextDecoder
		*TextEncoder
	}
)

// Ensure the interfaces are implemented correctly
var (
	_ modules.Instance = &ModuleInstance{}
	_ modules.Module   = &RootModule{}
)

// New returns a pointer to a new RootModule instance
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance implements the modules.Module interface and returns
// a new instance for each VU.
func (*RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &ModuleInstance{
		vu:          vu,
		TextDecoder: &TextDecoder{},
		TextEncoder: &TextEncoder{},
	}
}

// Exports implements the modules.Instance interface and returns
// the exports of the JS module.
func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{Named: map[string]interface{}{
		"TextDecoder": mi.NewTextDecoder,
		"TextEncoder": mi.NewTextEncoder,
	}}
}

// NewTextDecoder is the JS constructor for the TextDecoder object.
func (mi *ModuleInstance) NewTextDecoder(call sobek.ConstructorCall) *sobek.Object {
	rt := mi.vu.Runtime()

	// Parse the label parameter - default to "utf-8" if undefined
	var label string
	if call.Argument(0) != nil && !sobek.IsUndefined(call.Argument(0)) {
		err := rt.ExportTo(call.Argument(0), &label)
		if err != nil {
			common.Throw(rt, NewError(RangeError, "unable to extract label from the first argument; reason: "+err.Error()))
		}
	}
	if label == "" {
		label = "utf-8"
	}

	// Parse the options parameter
	var options TextDecoderOptions
	if call.Argument(1) != nil && !sobek.IsUndefined(call.Argument(1)) {
		err := rt.ExportTo(call.Argument(1), &options)
		if err != nil {
			common.Throw(rt, err)
		}
	}

	td, err := NewTextDecoder(rt, label, options)
	if err != nil {
		common.Throw(rt, err)
	}

	return newTextDecoderObject(rt, td)
}

// NewTextEncoder is the JS constructor for the TextEncoder object.
func (mi *ModuleInstance) NewTextEncoder(_ sobek.ConstructorCall) *sobek.Object {
	return newTextEncoderObject(mi.vu.Runtime(), NewTextEncoder())
}

// newTextDecoderObject converts the given TextDecoder instance into a JS object.
//
// It is used by the TextDecoder constructor to convert the Go instance into a JS,
// and will also set the relevant properties as read-only as per the spec.
//
// In the event setting the properties on the object where to fail, the function
// will throw a JS exception.
func newTextDecoderObject(rt *sobek.Runtime, td *TextDecoder) *sobek.Object {
	obj := rt.NewObject()

	// Wrap the Go TextDecoder.Decode method in a JS function
	decodeMethod := func(call sobek.FunctionCall) sobek.Value {
		// Handle variable arguments - buffer can be undefined/missing
		var buffer sobek.Value
		if len(call.Arguments) > 0 {
			buffer = call.Arguments[0]
		}

		// Handle options parameter
		var options TextDecodeOptions
		if len(call.Arguments) > 1 && !sobek.IsUndefined(call.Arguments[1]) {
			err := rt.ExportTo(call.Arguments[1], &options)
			if err != nil {
				common.Throw(rt, err)
			}
		}

		data, err := exportArrayBuffer(rt, buffer)
		if err != nil {
			common.Throw(rt, err)
		}

		decoded, err := td.Decode(data, options)
		if err != nil {
			// Check if it's our custom error type for proper JavaScript error throwing
			var encErr *Error
			if errors.As(err, &encErr) {
				// Throw the specific JavaScript error type
				switch encErr.Name {
				case TypeError:
					panic(rt.NewTypeError(encErr.Message))
				case RangeError:
					// Create a RangeError using the constructor
					rangeErrorConstructor := rt.Get("RangeError")
					rangeError, _ := rt.New(rangeErrorConstructor, rt.ToValue(encErr.Message))
					panic(rangeError)
				}
			}
			common.Throw(rt, err)
		}

		return rt.ToValue(decoded)
	}

	// Set the decode method to the wrapper function we just created
	if err := setReadOnlyPropertyOf(obj, "decode", rt.ToValue(decodeMethod)); err != nil {
		common.Throw(
			rt,
			errors.New("unable to define decode read-only property on TextDecoder object; reason: "+err.Error()),
		)
	}

	// Set the encoding property
	if err := setReadOnlyPropertyOf(obj, "encoding", rt.ToValue(td.Encoding)); err != nil {
		common.Throw(
			rt,
			errors.New("unable to define encoding read-only property on TextDecoder object; reason: "+err.Error()),
		)
	}

	// Set the fatal property
	if err := setReadOnlyPropertyOf(obj, "fatal", rt.ToValue(td.Fatal)); err != nil {
		common.Throw(
			rt,
			errors.New("unable to define fatal read-only property on TextDecoder object; reason: "+err.Error()),
		)
	}

	// Set the ignoreBOM property
	if err := setReadOnlyPropertyOf(obj, "ignoreBOM", rt.ToValue(td.IgnoreBOM)); err != nil {
		common.Throw(
			rt,
			errors.New("unable to define ignoreBOM read-only property on TextDecoder object; reason: "+err.Error()),
		)
	}

	return obj
}

func newTextEncoderObject(rt *sobek.Runtime, te *TextEncoder) *sobek.Object {
	obj := rt.NewObject()

	// Wrap the Go TextEncoder.Encode method in a JS function
	encodeMethod := func(s sobek.Value) *sobek.Object {
		// Handle undefined/null values by defaulting to empty string
		var text string
		if s == nil || sobek.IsUndefined(s) {
			text = ""
		} else {
			text = s.String()
		}

		buffer, err := te.Encode(text)
		if err != nil {
			common.Throw(rt, err)
		}

		// Create a new Uint8Array from the buffer
		u, err := rt.New(rt.Get("Uint8Array"), rt.ToValue(rt.NewArrayBuffer(buffer)))
		if err != nil {
			common.Throw(rt, err)
		}

		return u
	}

	// Set the encode property by wrapping the Go function in a JS function
	if err := setReadOnlyPropertyOf(obj, "encode", rt.ToValue(encodeMethod)); err != nil {
		common.Throw(
			rt,
			errors.New("unable to define encode read-only method on TextEncoder object; reason: "+err.Error()),
		)
	}

	// Set the encoding property
	if err := setReadOnlyPropertyOf(obj, "encoding", rt.ToValue(te.Encoding)); err != nil {
		common.Throw(
			rt,
			errors.New("unable to define encoding read-only property on TextEncoder object; reason: "+err.Error()),
		)
	}

	return obj
}
