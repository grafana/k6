package msgpack

import (
	"github.com/vmihailenco/msgpack/v5"
	"go.k6.io/k6-extension-api"
)

func init() {
	extensionapi.Register("k6/x/msgpack", new(MsgPackRoot))
}

// MsgPackRoot is the root module
type MsgPackRoot struct{}

type ModuleInstance struct {
	vu extensionapi.VU
}

func (*MsgPackRoot) NewModuleInstance(vu extensionapi.VU) extensionapi.Instance {
	return &ModuleInstance{vu: vu}
}

func (mi *ModuleInstance) Exports() extensionapi.Exports {
	return extensionapi.Exports{Default: &MessagePack{vu: mi.vu}}
}

// MessagePack is the k6 extension imported by the JavaScript
type MessagePack struct {
	vu extensionapi.VU
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
