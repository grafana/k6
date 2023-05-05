package fasthttp

import (
	"io"
)

type userDataKV struct {
	key   interface{}
	value interface{}
}

type userData []userDataKV

func (d *userData) Set(key interface{}, value interface{}) {
	if b, ok := key.([]byte); ok {
		key = string(b)
	}
	args := *d
	n := len(args)
	for i := 0; i < n; i++ {
		kv := &args[i]
		if kv.key == key {
			kv.value = value
			return
		}
	}

	if value == nil {
		return
	}

	c := cap(args)
	if c > n {
		args = args[:n+1]
		kv := &args[n]
		kv.key = key
		kv.value = value
		*d = args
		return
	}

	kv := userDataKV{}
	kv.key = key
	kv.value = value
	*d = append(args, kv)
}

func (d *userData) SetBytes(key []byte, value interface{}) {
	d.Set(key, value)
}

func (d *userData) Get(key interface{}) interface{} {
	if b, ok := key.([]byte); ok {
		key = b2s(b)
	}
	args := *d
	n := len(args)
	for i := 0; i < n; i++ {
		kv := &args[i]
		if kv.key == key {
			return kv.value
		}
	}
	return nil
}

func (d *userData) GetBytes(key []byte) interface{} {
	return d.Get(key)
}

func (d *userData) Reset() {
	args := *d
	n := len(args)
	for i := 0; i < n; i++ {
		v := args[i].value
		if vc, ok := v.(io.Closer); ok {
			vc.Close()
		}
	}
	*d = (*d)[:0]
}

func (d *userData) Remove(key interface{}) {
	if b, ok := key.([]byte); ok {
		key = b2s(b)
	}
	args := *d
	n := len(args)
	for i := 0; i < n; i++ {
		kv := &args[i]
		if kv.key == key {
			n--
			args[i], args[n] = args[n], args[i]
			args[n].value = nil
			args = args[:n]
			*d = args
			return
		}
	}
}

func (d *userData) RemoveBytes(key []byte) {
	d.Remove(key)
}
