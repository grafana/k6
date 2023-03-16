package wasm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/dop251/goja"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
)

type (
	RootModule struct{}
	Wasm       struct {
		vu modules.VU
	}
)

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &Wasm{}
)

func New() *RootModule {
	return &RootModule{}
}

func (*RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &Wasm{vu: vu}
}

func (w *Wasm) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]interface{}{
			"instantiate": w.instantiate,
		},
	}
}

func (w *Wasm) fetch(c context.Context, mod api.Module, methodptr, methodsz, urlptr, urlsz, bodyptr, bodysz, optptr, optsz uint32) uint32 {
	method, url, body, opts := "GET", "", []byte{}, []byte{}
	if m, ok := mod.Memory().Read(methodptr, methodsz); !ok {
		return 0
	} else if u, ok := mod.Memory().Read(urlptr, urlsz); !ok {
		return 0
	} else if b, ok := mod.Memory().Read(bodyptr, bodysz); !ok {
		return 0
	} else if o, ok := mod.Memory().Read(optptr, optsz); !ok {
		return 0
	} else {
		method = string(m)
		url = string(u)
		body = b
		opts = o
	}
	param := struct {
		Headers map[string]string `json:"headers"`
	}{Headers: map[string]string{}}
	if len(opts) > 0 {
		if err := json.Unmarshal(opts, &param); err != nil {
			log.Println(err)
			return 0
		}
	}
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		log.Println(err)
		return 0
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println(err)
		return 0
	}
	return uint32(res.StatusCode)
}

func (w *Wasm) instantiate(url string) (goja.Value, error) {
	if !strings.HasPrefix(url, "file://") && strings.Contains(url, "://") {
		return nil, errors.New("remote fetching of WASM is not implemented yet")
	}
	b, err := ioutil.ReadFile(strings.TrimPrefix(url, "file://"))
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	r := wazero.NewRuntime(ctx)
	_, err = r.NewHostModuleBuilder("env").NewFunctionBuilder().
		WithFunc(w.fetch).Export("fetch").Instantiate(ctx)
	if err != nil {
		return nil, err
	}
	wasi_snapshot_preview1.MustInstantiate(ctx, r)
	cfg := wazero.NewModuleConfig().WithStdout(os.Stdout).WithStderr(os.Stderr)
	mod, err := r.InstantiateWithConfig(ctx, b, cfg)
	if err != nil {
		return nil, err
	}
	rt := w.vu.Runtime()
	jsmod := rt.NewObject()
	exports := rt.NewObject()
	for _, def := range mod.ExportedFunctionDefinitions() {
		name := def.Name()
		fn := mod.ExportedFunction(name)
		params := def.ParamTypes()
		rets := def.ResultTypes()
		exports.Set(name, func(call goja.FunctionCall) goja.Value {
			if len(params) > len(call.Arguments) {
				common.Throw(rt, fmt.Errorf("%s: bad number of args: expected %d, got %d", name, len(params), len(call.Arguments)))
			}
			if len(rets) > 1 {
				common.Throw(rt, fmt.Errorf("%s: multiple returns are not supported", name))
			}

			args := make([]uint64, len(params))
			for i, p := range params {
				arg := call.Arguments[i]
				switch p {
				case api.ValueTypeI32:
					args[i] = api.EncodeI32(int32(arg.ToInteger()))
				case api.ValueTypeI64:
					args[i] = api.EncodeI64(arg.ToInteger())
				case api.ValueTypeF32:
					args[i] = api.EncodeF32(float32(arg.ToFloat()))
				case api.ValueTypeF64:
					args[i] = api.EncodeF64(arg.ToFloat())
				case api.ValueTypeExternref:
					args[i] = api.EncodeExternref(uintptr(arg.ToInteger()))
				default:
					common.Throw(rt, fmt.Errorf("%s: unexpected param %d type", name, i))
				}
			}
			resultData, err := fn.Call(ctx, args...)
			if err != nil {
				common.Throw(rt, fmt.Errorf("%s: %v", name, err))
			}
			if len(rets) == 0 {
				return goja.Undefined()
			}

			res := resultData[0]
			switch rets[0] {
			case api.ValueTypeI32:
				return rt.ToValue(api.DecodeI32(res))
			case api.ValueTypeI64:
				return rt.ToValue(int64(res))
			case api.ValueTypeF32:
				return rt.ToValue(api.DecodeF32(res))
			case api.ValueTypeF64:
				return rt.ToValue(api.DecodeF64(res))
			case api.ValueTypeExternref:
				return rt.ToValue(api.DecodeExternref(res))
			}
			return goja.Undefined()
		})
	}
	jsmod.Set("exports", exports)
	return jsmod, nil
}
