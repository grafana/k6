package encoding

import (
	"encoding/base64"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
)

type (
	// RootModule is the global module instance that will create module
	// instances for each VU.
	RootModule struct{}

	// Encoding represents an instance of the encoding module.
	Encoding struct {
		vu modules.VU
	}
)

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &Encoding{}
)

// New returns a pointer to a new RootModule instance.
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance implements the modules.Module interface to return
// a new instance for each VU.
func (*RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &Encoding{vu: vu}
}

// Exports returns the exports of the encoding module.
func (e *Encoding) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]interface{}{
			"b64encode": e.b64Encode,
			"b64decode": e.b64Decode,
		},
	}
}

// b64encode returns the base64 encoding of input as a string.
// The data type of input can be a string, []byte or ArrayBuffer.
func (e *Encoding) b64Encode(input interface{}, encoding string) string {
	data, err := common.ToBytes(input)
	if err != nil {
		common.Throw(e.vu.Runtime(), err)
	}
	switch encoding {
	case "rawstd":
		return base64.StdEncoding.WithPadding(base64.NoPadding).EncodeToString(data)
	case "std":
		return base64.StdEncoding.EncodeToString(data)
	case "rawurl":
		return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(data)
	case "url":
		return base64.URLEncoding.EncodeToString(data)
	default:
		return base64.StdEncoding.EncodeToString(data)
	}
}

// b64decode returns the decoded data of the base64 encoded input string using
// the given encoding. If format is "s" it returns the data as a string,
// otherwise as an ArrayBuffer.
func (e *Encoding) b64Decode(input, encoding, format string) interface{} {
	var (
		output []byte
		err    error
	)

	switch encoding {
	case "rawstd":
		output, err = base64.StdEncoding.WithPadding(base64.NoPadding).DecodeString(input)
	case "std":
		output, err = base64.StdEncoding.DecodeString(input)
	case "rawurl":
		output, err = base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(input)
	case "url":
		output, err = base64.URLEncoding.DecodeString(input)
	default:
		output, err = base64.StdEncoding.DecodeString(input)
	}

	if err != nil {
		common.Throw(e.vu.Runtime(), err)
	}

	var out interface{}
	if format == "s" {
		out = string(output)
	} else {
		ab := e.vu.Runtime().NewArrayBuffer(output)
		out = &ab
	}

	return out
}
