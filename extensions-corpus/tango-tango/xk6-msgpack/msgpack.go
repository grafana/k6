package msgpack

import (
	"github.com/vmihailenco/msgpack/v5"
	"go.k6.io/k6/js/modules"
)

func init() {
	modules.Register("k6/x/msgpack", new(MsgPackRoot))
}

// MsgPackRoot is the root module
type MsgPackRoot struct{}

type ModuleInstance struct {
	vu modules.VU
}

func (*MsgPackRoot) NewModuleInstance(vu modules.VU) modules.Instance {
	return &ModuleInstance{vu: vu}
}

func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{Default: &MessagePack{vu: mi.vu}}
}

// MessagePack is the k6 extension imported by the JavaScript
type MessagePack struct {
	vu modules.VU
}

// Pack encodes a value to MessagePack binary format and returns as ArrayBuffer
func (m *MessagePack) Pack(data interface{}) (interface{}, error) {
	encoded, err := msgpack.Marshal(data)
	if err != nil {
		return nil, err
	}

	rt := m.vu.Runtime()
	arrayBuffer := rt.NewArrayBuffer(encoded)

	// Return the ArrayBuffer as a sobek.Value
	return rt.ToValue(arrayBuffer).Export(), nil
}

// Unpack decodes MessagePack binary data to a value
func (m *MessagePack) Unpack(data []byte) (interface{}, error) {
	var result interface{}
	err := msgpack.Unmarshal(data, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}
